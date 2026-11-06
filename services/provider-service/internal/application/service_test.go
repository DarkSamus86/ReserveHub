package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestCheckTenantLifecycle(t *testing.T) {
	id := uuid.New()
	for _, tc := range []struct {
		name     string
		status   string
		admin    bool
		mutation bool
		want     error
	}{
		{"owner reads pending", "PENDING", false, false, nil},
		{"owner reads suspended", "SUSPENDED", false, false, nil},
		{"owner cannot read archived", "ARCHIVED", false, false, ErrTenantArchived},
		{"admin reads archived", "ARCHIVED", true, false, nil},
		{"admin cannot mutate suspended", "SUSPENDED", true, true, ErrTenantInactive},
		{"owner mutates active", "ACTIVE", false, true, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			svc := Service{Tenant: func(_ context.Context, got uuid.UUID, bearer string) (string, error) {
				calls++
				if got != id || bearer != "jwt" {
					t.Fatal("unexpected tenant lookup input")
				}
				return tc.status, nil
			}}
			got := svc.CheckTenant(context.Background(), id, "jwt", tc.admin, tc.mutation)
			if !errors.Is(got, tc.want) || calls != 1 {
				t.Fatalf("got %v, calls %d; want %v", got, calls, tc.want)
			}
		})
	}
}
