package gateway

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"darksamus.com/ReserveHub/api-gateway/internal/config"
)

const maxAuthBodyBytes = 1 << 20

type Gateway struct {
	verifyKey       ed25519.PublicKey
	upstream        *url.URL
	client          *http.Client
	proxy           *httputil.ReverseProxy
	logger          *slog.Logger
	ipLimiter       *keyedLimiter
	loginLimiter    *keyedLimiter
	registerLimiter *keyedLimiter
	subjectLimiter  *keyedLimiter
}

func New(cfg config.Config, logger *slog.Logger) (http.Handler, func(), error) {
	g := &Gateway{
		verifyKey: append(ed25519.PublicKey(nil), cfg.JWTVerifyKey...),
		upstream:  cfg.AuthServiceURL,
		logger:    logger,
		client: &http.Client{
			Timeout: 3 * time.Second,
			Transport: &http.Transport{
				Proxy:                 nil,
				DialContext:           (&net.Dialer{Timeout: 2 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
				ResponseHeaderTimeout: 2 * time.Second,
				TLSHandshakeTimeout:   2 * time.Second,
				IdleConnTimeout:       30 * time.Second,
			},
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		ipLimiter:       newKeyedLimiter(2, 60, 10000),
		loginLimiter:    newKeyedLimiter(1.0/6, 10, 10000),
		registerLimiter: newKeyedLimiter(1.0/60, 5, 10000),
		subjectLimiter:  newKeyedLimiter(2, 120, 10000),
	}
	g.proxy = g.newAuthProxy()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", g.live)
	mux.HandleFunc("GET /health/ready", g.ready)
	mux.HandleFunc("POST /v1/auth/register", g.register)
	mux.HandleFunc("POST /v1/auth/login", g.login)
	mux.HandleFunc("GET /v1/auth/me", g.me)
	mux.HandleFunc("/", g.notFound)
	return g.middleware(mux), func() {}, nil
}

func (g *Gateway) newAuthProxy() *httputil.ReverseProxy {
	proxy := &httputil.ReverseProxy{
		Transport: &http.Transport{
			Proxy:                 nil,
			DialContext:           (&net.Dialer{Timeout: 2 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   20,
			IdleConnTimeout:       60 * time.Second,
			TLSHandshakeTimeout:   3 * time.Second,
			ResponseHeaderTimeout: 8 * time.Second,
			ExpectContinueTimeout: time.Second,
		},
		Rewrite: func(request *httputil.ProxyRequest) {
			request.SetURL(g.upstream)
			request.SetXForwarded()
			for _, name := range []string{
				"Forwarded", "X-Forwarded-Host", "X-Forwarded-Proto", "X-Forwarded-Port",
				"X-User-ID", "X-User-Role", "X-Tenant-ID", "X-Authenticated-User",
				"X-Authenticated-Roles", "X-Auth-Request-User", "X-Auth-Request-Email", "X-Remote-User",
			} {
				request.Out.Header.Del(name)
			}
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			g.logger.Warn("auth upstream unavailable", "error", err)
			writeError(w, http.StatusBadGateway, "upstream_unavailable", "authentication service is temporarily unavailable")
		},
	}
	return proxy
}

func (g *Gateway) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := newRequestID()
		w.Header().Set("X-Request-ID", requestID)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		g.logger.Info("http request", "request_id", requestID, "method", r.Method, "path", r.URL.Path, "status", recorder.status, "duration_ms", time.Since(start).Milliseconds())
	})
}

func (g *Gateway) live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (g *Gateway) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(g.upstream.String(), "/")+"/health/ready", nil)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "authentication service is unavailable")
		return
	}
	response, err := g.client.Do(request)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "authentication service is unavailable")
		return
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "authentication service is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (g *Gateway) register(w http.ResponseWriter, r *http.Request) {
	if !g.allowIP(w, r) || !g.allow(g.registerLimiter, "register:"+clientIP(r)) {
		return
	}
	g.proxyAuth(w, r, false)
}

func (g *Gateway) login(w http.ResponseWriter, r *http.Request) {
	if !g.allowIP(w, r) || !g.allow(g.loginLimiter, "login:"+clientIP(r)) {
		return
	}
	g.proxyAuth(w, r, false)
}

func (g *Gateway) me(w http.ResponseWriter, r *http.Request) {
	if !g.allowIP(w, r) {
		return
	}
	token, err := bearerToken(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "valid bearer token required")
		return
	}
	claims, err := verifyToken(g.verifyKey, token, time.Now())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "invalid or expired bearer token")
		return
	}
	if !g.allow(g.subjectLimiter, "subject:"+claims.Subject) {
		writeRateLimited(w)
		return
	}
	g.proxyAuth(w, r, true)
}

func (g *Gateway) proxyAuth(w http.ResponseWriter, r *http.Request, authenticated bool) {
	if r.URL.RawQuery != "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "query parameters are not accepted on this endpoint")
		return
	}
	if r.Method == http.MethodPost {
		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "application/json" {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "content-type must be application/json")
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, maxAuthBodyBytes+1))
		if err != nil || len(body) > maxAuthBodyBytes {
			writeError(w, http.StatusRequestEntityTooLarge, "request_too_large", "request body exceeds the allowed size")
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		r.ContentLength = int64(len(body))
	} else if r.ContentLength > 0 || len(r.TransferEncoding) > 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body is not accepted")
		return
	}
	r.Header.Set("X-Request-ID", w.Header().Get("X-Request-ID"))
	r.Header.Del("Forwarded")
	r.Header.Del("X-Forwarded-For")
	r.Header.Del("X-Forwarded-Host")
	r.Header.Del("X-Forwarded-Proto")
	if !authenticated {
		r.Header.Del("Authorization")
	}
	g.proxy.ServeHTTP(w, r)
}

func (g *Gateway) notFound(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotFound, "not_found", "route not found")
}

func (g *Gateway) allowIP(w http.ResponseWriter, r *http.Request) bool {
	if !g.allow(g.ipLimiter, "ip:"+clientIP(r)) {
		writeRateLimited(w)
		return false
	}
	return true
}

func (g *Gateway) allow(limiter *keyedLimiter, key string) bool {
	return limiter.allow(key, time.Now())
}

func bearerToken(r *http.Request) (string, error) {
	values := r.Header.Values("Authorization")
	if len(values) != 1 {
		return "", errInvalidToken
	}
	parts := strings.Fields(values[0])
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", errInvalidToken
	}
	return parts[1], nil
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || host == "" {
		return "unknown"
	}
	return host
}

func newRequestID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(raw[:])
}

func writeRateLimited(w http.ResponseWriter) {
	w.Header().Set("Retry-After", "1")
	writeError(w, http.StatusTooManyRequests, "rate_limited", "too many requests")
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"code": code, "message": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (w *statusRecorder) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusRecorder) Write(body []byte) (int, error) {
	return w.ResponseWriter.Write(body)
}

func (w *statusRecorder) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}
