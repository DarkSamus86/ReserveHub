package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

const (
	testTenantID    = "11111111-1111-4111-8111-111111111111"
	testOperationID = "22222222-2222-4222-8222-222222222222"
)

func validProvisionInput() ProvisionOwnerInput {
	return ProvisionOwnerInput{
		TenantID: testTenantID, Slug: "9-" + strings.Repeat("a", 98),
		Email: " Owner@Example.com ", OperationID: testOperationID,
	}
}

func TestProvisionValidatesAndNormalizesInput(t *testing.T) {
	repo := &memoryRepository{provisionResult: ProvisionOwnerResult{TenantID: testTenantID, Status: "pending_delivery"}}
	service := NewService(repo, signingKey(t), time.Minute)
	input := validProvisionInput()
	if _, err := service.ProvisionOwner(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	if repo.provisionInput.Email != "owner@example.com" || repo.provisionInput.Slug != input.Slug || repo.provisionCalls != 1 {
		t.Fatalf("unexpected normalized input: %+v, calls=%d", repo.provisionInput, repo.provisionCalls)
	}
	for _, bad := range []ProvisionOwnerInput{
		{TenantID: "not-uuid", Slug: "tenant", Email: "a@example.com", OperationID: testOperationID},
		{TenantID: testTenantID, Slug: "bad--slug", Email: "a@example.com", OperationID: testOperationID},
		{TenantID: testTenantID, Slug: "tenant", Email: "a@example.com", OperationID: "not-uuid"},
		{TenantID: testTenantID, Slug: "tenant", Email: "invalid", OperationID: testOperationID},
	} {
		if _, err := service.ProvisionOwner(context.Background(), bad); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("input %+v: expected invalid input, got %v", bad, err)
		}
	}
	if repo.provisionCalls != 1 {
		t.Fatalf("invalid inputs reached repository: calls=%d", repo.provisionCalls)
	}
}
