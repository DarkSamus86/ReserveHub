package provision

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"darksamus.com/ReserveHub/admin-service/internal/repository"
	tenantv1 "darksamus.com/ReserveHub/tenant-service/gen/tenant/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type Input struct {
	Name            string `json:"name"`
	Slug            string `json:"slug"`
	Timezone        string `json:"timezone"`
	CurrencyCode    string `json:"currency_code"`
	LocaleCode      string `json:"locale_code"`
	DefaultLanguage string `json:"default_language"`
	PlanID          string `json:"plan_id"`
	OwnerEmail      string `json:"owner_email"`
}

type Result struct {
	OperationID             string     `json:"operation_id"`
	TenantID                string     `json:"tenant_id,omitempty"`
	Status                  string     `json:"status"`
	InviteStatus            string     `json:"invite_status,omitempty"`
	InviteExpiresAt         *time.Time `json:"invite_expires_at,omitempty"`
	InviteStatusUnavailable bool       `json:"invite_status_unavailable,omitempty"`
	Error                   string     `json:"error,omitempty"`
}

func resultFromOperation(op repository.Operation) Result {
	result := Result{OperationID: op.ID.String(), Status: op.Status, InviteStatus: op.InviteStatus, Error: op.SafeError}
	if op.TenantID.Valid {
		result.TenantID = op.TenantID.UUID.String()
	}
	return result
}

type Service struct {
	store        *repository.Store
	tenant       tenantv1.TenantServiceClient
	authURL      string
	client       *http.Client
	serviceToken string
}

func New(store *repository.Store, tenant tenantv1.TenantServiceClient, authURL, serviceToken string) *Service {
	return &Service{
		store: store, tenant: tenant, authURL: strings.TrimRight(authURL, "/"), serviceToken: serviceToken,
		client: &http.Client{
			Timeout:       5 * time.Second,
			Transport:     &http.Transport{Proxy: nil},
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (Result, error) {
	op, err := s.store.Get(ctx, id)
	if err != nil {
		return Result{}, err
	}
	return s.liveResult(ctx, op)
}

var ErrReissueUnavailable = errors.New("invitation cannot be reissued")

func (s *Service) liveResult(ctx context.Context, op repository.Operation) (Result, error) {
	result := resultFromOperation(op)
	if !op.InvitationID.Valid {
		return result, nil
	}
	unavailable := func() (Result, error) {
		result.InviteStatusUnavailable = true
		return result, nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet,
		s.authURL+"/internal/v1/tenant-invitations/"+op.InvitationID.UUID.String(), nil)
	if err != nil {
		return unavailable()
	}
	request.Header.Set("X-Service-Token", s.serviceToken)
	response, err := s.client.Do(request)
	if err != nil {
		return unavailable()
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return unavailable()
	}
	var state struct {
		InvitationID string     `json:"invitation_id"`
		Status       string     `json:"status"`
		ExpiresAt    *time.Time `json:"expires_at"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&state); err != nil {
		return unavailable()
	}
	if state.InvitationID != op.InvitationID.UUID.String() || state.Status == "" {
		return unavailable()
	}
	result.InviteStatus = strings.ToLower(state.Status)
	result.InviteExpiresAt = state.ExpiresAt
	return result, nil
}

func (s *Service) ReissueInvitation(ctx context.Context, opID, actor uuid.UUID) error {
	op, err := s.store.Get(ctx, opID)
	if err != nil {
		return err
	}
	if op.Status != "completed" || !op.InvitationID.Valid {
		return ErrReissueUnavailable
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost,
		s.authURL+"/internal/v1/tenant-invitations/"+op.InvitationID.UUID.String()+"/reissue", nil)
	if err != nil {
		return err
	}
	request.Header.Set("X-Service-Token", s.serviceToken)
	response, callErr := s.client.Do(request)
	result := "unavailable"
	if callErr == nil {
		defer response.Body.Close()
		switch response.StatusCode {
		case http.StatusAccepted:
			result = "accepted"
		case http.StatusConflict:
			result = "rejected"
		}
	}
	if err := s.store.RecordInvitationReissue(ctx, op, actor, result); err != nil {
		return err
	}
	if callErr != nil {
		return fmt.Errorf("reissue invitation: %w", callErr)
	}
	if response.StatusCode != http.StatusAccepted {
		if response.StatusCode == http.StatusConflict {
			return ErrReissueUnavailable
		}
		return fmt.Errorf("reissue invitation: Auth returned %d", response.StatusCode)
	}
	return nil
}

func (s *Service) Run(ctx context.Context, opID, actor uuid.UUID, hash, requestJSON []byte, input Input, bearer string) (Result, int, error) {
	tx, err := s.store.Begin(ctx, opID, actor, hash, requestJSON, input.OwnerEmail)
	if err != nil {
		return Result{OperationID: opID.String()}, 0, err
	}
	defer tx.Rollback(context.Background())
	op := resultFromOperation(tx.Operation)
	if !bytes.Equal(tx.Operation.RequestHash, hash) {
		return Result{OperationID: opID.String(), Status: "conflict", Error: "idempotency_key_reused"}, 409, nil
	}
	if op.Status == "completed" {
		live, err := s.liveResult(ctx, tx.Operation)
		return live, 200, err
	}
	if op.Status == "failed" {
		return op, 409, nil
	}
	if !tx.Operation.TenantID.Valid {
		callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		callCtx = metadata.AppendToOutgoingContext(callCtx, "authorization", "Bearer "+bearer, "x-service-token", s.serviceToken)
		created, callErr := s.tenant.CreateTenant(callCtx, &tenantv1.CreateTenantRequest{
			Name: input.Name, Slug: input.Slug, Timezone: input.Timezone, CurrencyCode: input.CurrencyCode,
			LocaleCode: input.LocaleCode, DefaultLanguage: input.DefaultLanguage, PlanId: input.PlanID, OperationId: opID.String(),
		})
		cancel()
		if callErr != nil {
			code := status.Code(callErr)
			if code == codes.InvalidArgument || code == codes.AlreadyExists || code == codes.FailedPrecondition {
				op.Status = "failed"
				op.Error = "tenant_rejected"
			} else {
				op.Status = "retryable_failure"
				op.Error = "tenant_unavailable"
			}
			if err := saveAndCommit(ctx, tx, op); err != nil {
				return op, 0, err
			}
			if op.Status == "failed" {
				return op, 422, nil
			}
			return op, 503, nil
		}
		tenantID, err := uuid.Parse(created.GetId())
		if err != nil {
			return op, 0, err
		}
		if err := tx.SetTenant(ctx, tenantID); err != nil {
			return op, 0, err
		}
		op.TenantID = tenantID.String()
	}
	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	body, _ := json.Marshal(map[string]string{"tenant_id": op.TenantID, "slug": input.Slug, "email": input.OwnerEmail, "operation_id": opID.String()})
	req, err := http.NewRequestWithContext(callCtx, http.MethodPost, s.authURL+"/internal/v1/tenant-memberships/provision-owner", bytes.NewReader(body))
	if err != nil {
		cancel()
		return op, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-Token", s.serviceToken)
	res, callErr := s.client.Do(req)
	if callErr == nil {
		defer res.Body.Close()
		var authResult struct {
			Status       string `json:"status"`
			InvitationID string `json:"invitation_id"`
		}
		if res.StatusCode == 200 || res.StatusCode == 201 {
			callErr = json.NewDecoder(io.LimitReader(res.Body, 4096)).Decode(&authResult)
			invitationID, parseErr := uuid.Parse(authResult.InvitationID)
			if callErr == nil && authResult.Status != "" && parseErr == nil && invitationID != uuid.Nil {
				if err := tx.SetInvitation(ctx, invitationID); err != nil {
					cancel()
					return op, 0, err
				}
				op.Status = "completed"
				op.InviteStatus = authResult.Status
				op.Error = ""
			}
		} else if res.StatusCode == 409 || res.StatusCode == 400 {
			op.Status = "failed"
			op.Error = "owner_provision_rejected"
		}
	}
	cancel()
	if op.Status != "completed" && op.Status != "failed" {
		op.Status = "retryable_failure"
		op.Error = "owner_provision_unavailable"
	}
	if err := saveAndCommit(ctx, tx, op); err != nil {
		return op, 0, err
	}
	if op.Status == "completed" {
		return op, 201, nil
	}
	if op.Status == "failed" {
		return op, 409, nil
	}
	return op, 503, nil
}

func saveAndCommit(ctx context.Context, tx *repository.Transaction, op Result) error {
	if err := tx.SetResult(ctx, op.Status, op.InviteStatus, op.Error); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
