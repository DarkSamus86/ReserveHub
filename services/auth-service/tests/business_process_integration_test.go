package tests

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"darksamus.com/ReserveHub/auth-service/internal/auth"
	"darksamus.com/ReserveHub/auth-service/internal/httpapi"
	"darksamus.com/ReserveHub/auth-service/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type capturedInvitation struct {
	email string
	token string
}

type capturingSender struct{ sent chan capturedInvitation }

func (s *capturingSender) SendInvitation(_ context.Context, email, token string) error {
	s.sent <- capturedInvitation{email: email, token: token}
	return nil
}

type retryingSender struct {
	mu    sync.Mutex
	calls int
	sent  chan capturedInvitation
}

func (s *retryingSender) SendInvitation(_ context.Context, email, token string) error {
	s.mu.Lock()
	s.calls++
	call := s.calls
	s.mu.Unlock()
	s.sent <- capturedInvitation{email: email, token: token}
	if call == 1 {
		return errors.New("synthetic SMTP outage")
	}
	return nil
}

func qaDatabase(t *testing.T) (context.Context, *repository.Postgres, *pgxpool.Pool) {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	store, err := repository.NewPostgres(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		store.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Close(); store.Close() })
	return ctx, store, pool
}

func waitInvitationState(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id, want string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var status string
		if err := pool.QueryRow(ctx, `SELECT status FROM auth_service.tr_tenant_invitation WHERE id=$1`, id).Scan(&status); err != nil {
			t.Fatal(err)
		}
		if status == want {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("invitation %s did not reach %s", id, want)
}

// Other integration cases may leave pending invitations in the shared test DB.
// Lock them while this case runs so the delivery worker selects only its target.
func lockOtherInvitations(t *testing.T, ctx context.Context, pool *pgxpool.Pool, targetID string) pgx.Tx {
	t.Helper()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := tx.Query(ctx, `SELECT id FROM auth_service.tr_tenant_invitation WHERE id<>$1 FOR UPDATE`, targetID)
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	for rows.Next() {
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	rows.Close()
	return tx
}

func deliverInvitation(t *testing.T, ctx context.Context, store *repository.Postgres, pool *pgxpool.Pool, id, email string) string {
	t.Helper()
	otherInvitations := lockOtherInvitations(t, ctx, pool, id)
	defer otherInvitations.Rollback(ctx)
	workerCtx, cancel := context.WithCancel(ctx)
	sender := &capturingSender{sent: make(chan capturedInvitation, 1)}
	finished := make(chan error, 1)
	go func() {
		finished <- store.RunInvitationDelivery(workerCtx, []byte("01234567890123456789012345678901"), sender, 2*time.Minute)
	}()
	var invitation capturedInvitation
	select {
	case invitation = <-sender.sent:
	case <-time.After(10 * time.Second):
		cancel()
		t.Fatal("invitation was not delivered")
	}
	if invitation.email != email || len(invitation.token) != 43 {
		cancel()
		t.Fatalf("unexpected captured invitation for %q", invitation.email)
	}
	waitInvitationState(t, ctx, pool, id, "SENT")
	cancel()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("invitation worker did not stop")
	}
	return invitation.token
}

func TestQANewOwnerInvitationAcceptanceAndReplay(t *testing.T) {
	ctx, store, pool := qaDatabase(t)
	service := auth.NewService(store, signingKey(t), 15*time.Minute)
	email := "qa-new-" + uuid.NewString() + "@example.com"
	input := auth.ProvisionOwnerInput{TenantID: uuid.NewString(), OperationID: uuid.NewString(), Slug: "qa-owner-" + uuid.NewString(), Email: email}
	created, err := service.ProvisionOwner(ctx, input)
	if err != nil || !created.Created || created.Status != "pending_delivery" {
		t.Fatalf("provision owner: %+v, %v", created, err)
	}
	if _, err := service.Login(ctx, email, "qa password 123"); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("undelivered invitation granted login: %v", err)
	}
	var tokenHash []byte
	var membershipStatus, invitationStatus string
	if err := pool.QueryRow(ctx, `SELECT i.token_hash,i.status,m.status FROM auth_service.tr_tenant_invitation i JOIN auth_service.ln_user_tenant_membership m ON m.id=i.membership_id WHERE i.id=$1`, created.InvitationID).Scan(&tokenHash, &invitationStatus, &membershipStatus); err != nil {
		t.Fatal(err)
	}
	if tokenHash != nil || invitationStatus != "PENDING_DELIVERY" || membershipStatus != "PENDING" {
		t.Fatalf("owner was activated before delivery: %s/%s", invitationStatus, membershipStatus)
	}
	token := deliverInvitation(t, ctx, store, pool, created.InvitationID, email)
	if _, err := service.AcceptInvitation(ctx, token, ""); !errors.Is(err, auth.ErrInvalidSecret) {
		t.Fatalf("new owner accepted without choosing password: %v", err)
	}
	password := "qa owner password 123"
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := service.AcceptInvitation(ctx, token, password)
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	successes, denied := 0, 0
	for err := range results {
		if err == nil {
			successes++
		} else if errors.Is(err, auth.ErrInvalidSecret) {
			denied++
		} else {
			t.Fatalf("unexpected concurrent acceptance error: %v", err)
		}
	}
	if successes != 1 || denied != 1 {
		t.Fatalf("concurrent accept successes=%d denied=%d", successes, denied)
	}
	if _, err := service.AcceptInvitation(ctx, token, password); !errors.Is(err, auth.ErrInvalidSecret) {
		t.Fatalf("replayed invitation was accepted: %v", err)
	}
	var userStatus string
	if err := pool.QueryRow(ctx, `SELECT i.status,m.status,u.status FROM auth_service.tr_tenant_invitation i JOIN auth_service.ln_user_tenant_membership m ON m.id=i.membership_id JOIN auth_service.md_user u ON u.id=m.user_id WHERE i.id=$1`, created.InvitationID).Scan(&invitationStatus, &membershipStatus, &userStatus); err != nil {
		t.Fatal(err)
	}
	if invitationStatus != "ACCEPTED" || membershipStatus != "ACTIVE" || userStatus != "ACTIVE" {
		t.Fatalf("accept did not atomically activate owner: %s/%s/%s", invitationStatus, membershipStatus, userStatus)
	}
	if _, err := service.Login(ctx, email, password); err != nil {
		t.Fatalf("accepted owner cannot log in: %v", err)
	}
	rows, err := pool.Query(ctx, `SELECT payload::text FROM auth_service.sys_outbox_event WHERE aggregate_id=$1`, created.InvitationID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(payload, token) || strings.Contains(payload, email) || strings.Contains(payload, password) {
			t.Fatalf("outbox exposed owner secret or email: %s", payload)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestQAExistingOwnerKeepsPasswordAndRefreshExpiresAbsolutely(t *testing.T) {
	ctx, store, pool := qaDatabase(t)
	service := auth.NewService(store, signingKey(t), 15*time.Minute)
	email := "qa-existing-" + uuid.NewString() + "@example.com"
	password := "existing owner password 123"
	if _, err := service.Register(ctx, email, password); err != nil {
		t.Fatal(err)
	}
	input := auth.ProvisionOwnerInput{TenantID: uuid.NewString(), OperationID: uuid.NewString(), Slug: "qa-existing-" + uuid.NewString(), Email: email}
	created, err := service.ProvisionOwner(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	var originalHash string
	if err := pool.QueryRow(ctx, `SELECT password_hash FROM auth_service.md_user WHERE id=$1`, created.UserID).Scan(&originalHash); err != nil {
		t.Fatal(err)
	}
	token := deliverInvitation(t, ctx, store, pool, created.InvitationID, email)
	if _, err := service.AcceptInvitation(ctx, token, "replacement password 123"); !errors.Is(err, auth.ErrInvalidSecret) {
		t.Fatalf("existing owner password changed through invitation: %v", err)
	}
	if _, err := service.AcceptInvitation(ctx, token, ""); err != nil {
		t.Fatalf("existing owner cannot accept: %v", err)
	}
	var afterHash string
	if err := pool.QueryRow(ctx, `SELECT password_hash FROM auth_service.md_user WHERE id=$1`, created.UserID).Scan(&afterHash); err != nil {
		t.Fatal(err)
	}
	if afterHash != originalHash {
		t.Fatal("existing owner password hash changed")
	}
	login, err := service.Login(ctx, email, password)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Refresh(ctx, "invalid"); !errors.Is(err, auth.ErrInvalidSecret) {
		t.Fatalf("malformed refresh was accepted: %v", err)
	}
	refreshed, err := service.Refresh(ctx, login.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.RefreshExpiresAt.Unix() != login.RefreshExpiresAt.Unix() {
		t.Fatalf("refresh extended absolute session expiry: before=%s after=%s", login.RefreshExpiresAt, refreshed.RefreshExpiresAt)
	}
	if refreshed.RefreshToken == login.RefreshToken {
		t.Fatal("refresh token was not rotated")
	}
	if _, err := service.Refresh(ctx, login.RefreshToken); !errors.Is(err, auth.ErrInvalidSecret) {
		t.Fatalf("old refresh token remained valid: %v", err)
	}
	storedHash := sha256.Sum256([]byte(refreshed.RefreshToken))
	var databaseHash []byte
	if err := pool.QueryRow(ctx, `SELECT current_refresh_token_hash FROM auth_service.tr_auth_session WHERE id=$1`, strings.Split(refreshed.RefreshToken, ".")[0]).Scan(&databaseHash); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(databaseHash, storedHash[:]) {
		t.Fatal("refresh token hash mismatch in session")
	}
	if err := service.Logout(ctx, refreshed.RefreshToken); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Refresh(ctx, refreshed.RefreshToken); !errors.Is(err, auth.ErrInvalidSecret) {
		t.Fatalf("logout did not revoke refresh token: %v", err)
	}
	if err := service.Logout(ctx, refreshed.RefreshToken); !errors.Is(err, auth.ErrInvalidSecret) {
		t.Fatalf("logout accepted replay: %v", err)
	}
	secondLogin, err := service.Login(ctx, email, password)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	concurrent := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := service.Refresh(ctx, secondLogin.RefreshToken)
			concurrent <- err
		}()
	}
	wg.Wait()
	close(concurrent)
	successes, denied := 0, 0
	for err := range concurrent {
		if err == nil {
			successes++
		} else if errors.Is(err, auth.ErrInvalidSecret) {
			denied++
		} else {
			t.Fatalf("unexpected concurrent refresh error: %v", err)
		}
	}
	if successes != 1 || denied != 1 {
		t.Fatalf("concurrent refresh successes=%d denied=%d", successes, denied)
	}
	rows, err := pool.Query(ctx, `SELECT payload::text FROM auth_service.sys_outbox_event WHERE aggregate_id=$1`, strings.Split(secondLogin.RefreshToken, ".")[0])
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(payload, secondLogin.RefreshToken) || strings.Contains(payload, email) {
			t.Fatalf("session audit exposed refresh token or email: %s", payload)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	boundaryLogin, err := service.Login(ctx, email, password)
	if err != nil {
		t.Fatal(err)
	}
	boundaryID := strings.Split(boundaryLogin.RefreshToken, ".")[0]
	if _, err := pool.Exec(ctx, `UPDATE auth_service.tr_auth_session SET created_at=now()-interval '2 days',expires_at=now()-interval '1 day' WHERE id=$1`, boundaryID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Refresh(ctx, boundaryLogin.RefreshToken); !errors.Is(err, auth.ErrInvalidSecret) {
		t.Fatalf("expired refresh token was accepted: %v", err)
	}
}

func TestQAExpiredInvitationCanBeReissuedWithoutOldTokenReuse(t *testing.T) {
	ctx, store, pool := qaDatabase(t)
	service := auth.NewService(store, signingKey(t), 15*time.Minute)
	email := "qa-reissue-" + uuid.NewString() + "@example.com"
	input := auth.ProvisionOwnerInput{TenantID: uuid.NewString(), OperationID: uuid.NewString(), Slug: "qa-reissue-" + uuid.NewString(), Email: email}
	created, err := service.ProvisionOwner(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	oldToken := deliverInvitation(t, ctx, store, pool, created.InvitationID, email)
	if _, err := pool.Exec(ctx, `UPDATE auth_service.tr_tenant_invitation SET expires_at=now()-interval '1 second' WHERE id=$1`, created.InvitationID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AcceptInvitation(ctx, oldToken, "qa reissue password 123"); !errors.Is(err, auth.ErrInvalidSecret) {
		t.Fatalf("expired invitation was accepted: %v", err)
	}
	if err := service.ReissueInvitation(ctx, created.InvitationID); err != nil {
		t.Fatalf("reissue expired invitation: %v", err)
	}
	var status string
	var hash []byte
	if err := pool.QueryRow(ctx, `SELECT status,token_hash FROM auth_service.tr_tenant_invitation WHERE id=$1`, created.InvitationID).Scan(&status, &hash); err != nil {
		t.Fatal(err)
	}
	if status != "PENDING_DELIVERY" || hash != nil {
		t.Fatalf("reissue retained previous token: status=%s hash_present=%v", status, hash != nil)
	}
	var oldPayload string
	if err := pool.QueryRow(ctx, `SELECT payload::text FROM auth_service.sys_outbox_event WHERE aggregate_id=$1 AND event_type='auth.invitation.delivery.v1' ORDER BY created_at DESC LIMIT 1`, created.InvitationID).Scan(&oldPayload); err != nil {
		t.Fatal(err)
	}
	if oldPayload != "{}" {
		t.Fatalf("previous encrypted delivery command was retained: %s", oldPayload)
	}
	newToken := deliverInvitation(t, ctx, store, pool, created.InvitationID, email)
	if newToken == oldToken {
		t.Fatal("reissue repeated old token")
	}
	if _, err := service.AcceptInvitation(ctx, oldToken, "qa reissue password 123"); !errors.Is(err, auth.ErrInvalidSecret) {
		t.Fatalf("old invitation token accepted after reissue: %v", err)
	}
	if _, err := service.AcceptInvitation(ctx, newToken, "qa reissue password 123"); err != nil {
		t.Fatalf("new invitation token rejected: %v", err)
	}
}

func TestQAInvitationCannotBeAcceptedBeforeSMTPAcknowledgement(t *testing.T) {
	ctx, store, pool := qaDatabase(t)
	service := auth.NewService(store, signingKey(t), 15*time.Minute)
	email := "qa-smtp-" + uuid.NewString() + "@example.com"
	created, err := service.ProvisionOwner(ctx, auth.ProvisionOwnerInput{
		TenantID: uuid.NewString(), OperationID: uuid.NewString(), Slug: "qa-smtp-" + uuid.NewString(), Email: email,
	})
	if err != nil {
		t.Fatal(err)
	}
	otherInvitations := lockOtherInvitations(t, ctx, pool, created.InvitationID)
	defer otherInvitations.Rollback(ctx)
	workerCtx, cancel := context.WithCancel(ctx)
	sender := &retryingSender{sent: make(chan capturedInvitation, 2)}
	finished := make(chan error, 1)
	go func() {
		finished <- store.RunInvitationDelivery(workerCtx, []byte("01234567890123456789012345678901"), sender, 2*time.Minute)
	}()
	defer func() {
		cancel()
		select {
		case err := <-finished:
			if err != nil {
				t.Errorf("delivery worker stopped: %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Error("delivery worker did not stop")
		}
	}()
	var first capturedInvitation
	select {
	case first = <-sender.sent:
	case <-time.After(10 * time.Second):
		t.Fatal("SMTP failure path was not reached")
	}
	waitInvitationState(t, ctx, pool, created.InvitationID, "READY_TO_SEND")
	if _, err := service.AcceptInvitation(ctx, first.token, "qa smtp password 123"); !errors.Is(err, auth.ErrInvalidSecret) {
		t.Fatalf("invitation accepted before SMTP ACK: %v", err)
	}
	var encryptedPayload string
	if err := pool.QueryRow(ctx, `SELECT payload::text FROM auth_service.sys_outbox_event WHERE aggregate_id=$1 AND event_type='auth.invitation.delivery.v1'`, created.InvitationID).Scan(&encryptedPayload); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(encryptedPayload, first.token) || strings.Contains(encryptedPayload, email) {
		t.Fatalf("delivery outbox exposed plaintext: %s", encryptedPayload)
	}
	if _, err := pool.Exec(ctx, `UPDATE auth_service.sys_outbox_event SET next_attempt_at=now() WHERE aggregate_id=$1 AND event_type='auth.invitation.delivery.v1' AND published_at IS NULL`, created.InvitationID); err != nil {
		t.Fatal(err)
	}
	var second capturedInvitation
	select {
	case second = <-sender.sent:
	case <-time.After(6 * time.Second):
		t.Fatal("SMTP retry was not reached")
	}
	if second.token != first.token {
		t.Fatal("retry rotated token before previous invitation expired")
	}
	waitInvitationState(t, ctx, pool, created.InvitationID, "SENT")
	if _, err := service.AcceptInvitation(ctx, second.token, "qa smtp password 123"); err != nil {
		t.Fatalf("invitation rejected after SMTP ACK: %v", err)
	}
}

func TestQAAuthHTTPInvitationAndSessionContracts(t *testing.T) {
	ctx, store, pool := qaDatabase(t)
	service := auth.NewService(store, signingKey(t), 15*time.Minute)
	email := "qa-http-" + uuid.NewString() + "@example.com"
	created, err := service.ProvisionOwner(ctx, auth.ProvisionOwnerInput{
		TenantID: uuid.NewString(), OperationID: uuid.NewString(), Slug: "qa-http-" + uuid.NewString(), Email: email,
	})
	if err != nil {
		t.Fatal(err)
	}
	token := deliverInvitation(t, ctx, store, pool, created.InvitationID, email)
	handler := httpapi.New(service, slog.New(slog.NewTextHandler(io.Discard, nil)), "0123456789abcdef0123456789abcdef")
	request := func(method, path, body, serviceToken string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if serviceToken != "" {
			req.Header.Set("X-Service-Token", serviceToken)
		}
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		return res
	}
	statusPath := "/internal/v1/tenant-invitations/" + created.InvitationID
	if response := request(http.MethodGet, statusPath, "", ""); response.Code != http.StatusUnauthorized {
		t.Fatalf("internal invitation status was public: %d", response.Code)
	}
	response := request(http.MethodGet, statusPath, "", "0123456789abcdef0123456789abcdef")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"status":"SENT"`) || strings.Contains(response.Body.String(), token) {
		t.Fatalf("internal status contract: %d %s", response.Code, response.Body.String())
	}
	acceptBody, _ := json.Marshal(map[string]string{"token": token, "password": "qa http password 123"})
	response = request(http.MethodPost, "/v1/auth/invitations/accept", string(acceptBody), "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"status":"active"`) || strings.Contains(response.Body.String(), token) {
		t.Fatalf("accept contract: %d %s", response.Code, response.Body.String())
	}
	if response := request(http.MethodPost, "/v1/auth/invitations/accept", string(acceptBody), ""); response.Code != http.StatusUnauthorized {
		t.Fatalf("replayed accept HTTP code: %d", response.Code)
	}
	login, err := service.Login(ctx, email, "qa http password 123")
	if err != nil {
		t.Fatal(err)
	}
	refreshBody, _ := json.Marshal(map[string]string{"refresh_token": login.RefreshToken})
	response = request(http.MethodPost, "/v1/auth/refresh", string(refreshBody), "")
	if response.Code != http.StatusOK {
		t.Fatalf("refresh HTTP code: %d %s", response.Code, response.Body.String())
	}
	var refreshed auth.Session
	if err := json.Unmarshal(response.Body.Bytes(), &refreshed); err != nil || refreshed.RefreshToken == "" || refreshed.RefreshToken == login.RefreshToken {
		t.Fatalf("refresh HTTP body: %+v %v", refreshed, err)
	}
	logoutBody, _ := json.Marshal(map[string]string{"refresh_token": refreshed.RefreshToken})
	if response := request(http.MethodPost, "/v1/auth/logout", string(logoutBody), ""); response.Code != http.StatusNoContent {
		t.Fatalf("logout HTTP code: %d", response.Code)
	}
	if response := request(http.MethodPost, "/v1/auth/refresh", string(logoutBody), ""); response.Code != http.StatusUnauthorized {
		t.Fatalf("refresh after logout HTTP code: %d", response.Code)
	}
}
