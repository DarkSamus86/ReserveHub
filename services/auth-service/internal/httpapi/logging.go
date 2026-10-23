package httpapi

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

func LoggingMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := requestIDFrom(r)
		w.Header().Set("X-Request-ID", requestID)
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		started := time.Now()
		next.ServeHTTP(recorder, r)

		route := r.Pattern
		if route == "" {
			route = "unmatched"
		}
		level := slog.LevelInfo
		if recorder.status >= http.StatusInternalServerError {
			level = slog.LevelError
		} else if recorder.status >= http.StatusBadRequest {
			level = slog.LevelWarn
		}
		logger.LogAttrs(r.Context(), level, "http_request",
			slog.String("request_id", requestID),
			slog.String("http_method", r.Method),
			slog.String("http_route", route),
			slog.Int("http_status", recorder.status),
			slog.Float64("duration_ms", float64(time.Since(started))/float64(time.Millisecond)),
		)
	})
}

func requestIDFrom(r *http.Request) string {
	value := strings.TrimSpace(r.Header.Get("X-Request-ID"))
	if value == "" || len(value) > 128 {
		return uuid.NewString()
	}
	for _, char := range value {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || strings.ContainsRune("-_.:", char)) {
			return uuid.NewString()
		}
	}
	return value
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (w *statusRecorder) WriteHeader(status int) {
	if w.wrote {
		return
	}
	w.status = status
	w.wrote = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusRecorder) Write(data []byte) (int, error) {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}
