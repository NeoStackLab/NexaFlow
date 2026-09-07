package tenantcontext

import (
	"context"
	"errors"
)

var (
	ErrRequired       = errors.New("tenant context is required")
	ErrTenantMismatch = errors.New("tenant context does not match requested tenant")
)

type Scope struct {
	TenantID    string
	UserID      string
	PrimaryRole string
	Roles       []string
	Permissions []string
	System      bool
}

type contextKey struct{}

func With(ctx context.Context, scope Scope) context.Context {
	scope.Roles = append([]string(nil), scope.Roles...)
	scope.Permissions = append([]string(nil), scope.Permissions...)
	return context.WithValue(ctx, contextKey{}, scope)
}

func WithSystem(ctx context.Context) context.Context {
	return With(ctx, Scope{System: true})
}

func From(ctx context.Context) (Scope, bool) {
	if ctx == nil {
		return Scope{}, false
	}
	scope, ok := ctx.Value(contextKey{}).(Scope)
	return scope, ok
}

func Require(ctx context.Context) (Scope, error) {
	scope, ok := From(ctx)
	if !ok || scope.System || scope.TenantID == "" || scope.UserID == "" {
		return Scope{}, ErrRequired
	}
	return scope, nil
}

func RequireTenant(ctx context.Context, requested string) (Scope, error) {
	scope, err := Require(ctx)
	if err != nil {
		return Scope{}, err
	}
	if requested != "" && requested != scope.TenantID {
		return Scope{}, ErrTenantMismatch
	}
	return scope, nil
}

func HasPermission(ctx context.Context, permission string) bool {
	scope, ok := From(ctx)
	if !ok {
		return false
	}
	if scope.System {
		return true
	}
	for _, granted := range scope.Permissions {
		if granted == permission {
			return true
		}
	}
	return false
}
