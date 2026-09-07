package tenantcontext

import (
	"context"
	"errors"
	"testing"
)

func TestScopeIsImmutableAndRequired(t *testing.T) {
	roles := []string{"admin"}
	permissions := []string{"tenant.view"}
	ctx := With(context.Background(), Scope{TenantID: "tenant-1", UserID: "user-1", Roles: roles, Permissions: permissions})
	roles[0], permissions[0] = "guest", "other"

	scope, err := RequireTenant(ctx, "tenant-1")
	if err != nil {
		t.Fatalf("require tenant: %v", err)
	}
	if scope.Roles[0] != "admin" || scope.Permissions[0] != "tenant.view" || !HasPermission(ctx, "tenant.view") {
		t.Fatalf("unexpected scope: %+v", scope)
	}
	if _, err := RequireTenant(ctx, "tenant-2"); !errors.Is(err, ErrTenantMismatch) {
		t.Fatalf("expected tenant mismatch, got %v", err)
	}
	if _, err := Require(context.Background()); !errors.Is(err, ErrRequired) {
		t.Fatalf("expected required error, got %v", err)
	}
}

func TestSystemContextIsExplicitAndNotAUserTenant(t *testing.T) {
	ctx := WithSystem(context.Background())
	scope, ok := From(ctx)
	if !ok || !scope.System || !HasPermission(ctx, "anything") {
		t.Fatalf("unexpected system scope: %+v", scope)
	}
	if _, err := Require(ctx); !errors.Is(err, ErrRequired) {
		t.Fatalf("system context must not satisfy a tenant user requirement: %v", err)
	}
}
