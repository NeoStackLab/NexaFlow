package service

import (
	"context"
	"errors"
	"testing"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	"github.com/NeoStackLab/NexaFlow/backend/internal/tenantcontext"
)

type tenantRepositoryStub struct {
	updated    model.UpdateTenantInput
	settings   model.UpdateTenantSettingsInput
	transition string
}

func (*tenantRepositoryStub) Get(context.Context) (model.TenantView, error) {
	return model.TenantView{ID: "tenant-1"}, nil
}
func (s *tenantRepositoryStub) Update(_ context.Context, input model.UpdateTenantInput) (model.TenantView, error) {
	s.updated = input
	return model.TenantView{Name: input.Name}, nil
}
func (s *tenantRepositoryStub) Transition(_ context.Context, target string) (model.TenantView, error) {
	s.transition = target
	return model.TenantView{Status: target}, nil
}
func (*tenantRepositoryStub) Settings(context.Context) (model.TenantSettingsView, error) {
	return model.TenantSettingsView{}, nil
}
func (s *tenantRepositoryStub) UpdateSettings(_ context.Context, input model.UpdateTenantSettingsInput) (model.TenantSettingsView, error) {
	s.settings = input
	return model.TenantSettingsView{Timezone: input.Timezone}, nil
}

func TestTenantServiceRequiresContextAndValidatesSettings(t *testing.T) {
	repository := &tenantRepositoryStub{}
	service := NewTenantService(repository)
	if _, err := service.Get(context.Background()); !errors.Is(err, tenantcontext.ErrRequired) {
		t.Fatalf("expected tenant context error, got %v", err)
	}
	ctx := tenantcontext.With(context.Background(), tenantcontext.Scope{TenantID: "tenant-1", UserID: "user-1"})
	input := model.UpdateTenantSettingsInput{
		Timezone: "Asia/Shanghai", Locale: "zh-CN", CustomDomain: "APP.EXAMPLE.COM.", ExpectedVersion: 1,
		Branding: model.TenantBranding{PrimaryColor: "#3366FF", LogoURL: "https://cdn.example.com/logo.svg"},
		Security: model.TenantSecuritySettings{MinimumPasswordLength: 14, SessionTTLMinutes: 120, AllowedEmailDomains: []string{"example.com"}, IPAllowlist: []string{"203.0.113.0/24"}},
	}
	if _, err := service.UpdateSettings(ctx, input); err != nil {
		t.Fatalf("update settings: %v", err)
	}
	if repository.settings.CustomDomain != "app.example.com" {
		t.Fatalf("domain was not normalized: %q", repository.settings.CustomDomain)
	}
	if _, err := service.UpdateSettings(ctx, model.UpdateTenantSettingsInput{Timezone: "Mars/Olympus", Locale: "bad", ExpectedVersion: 1}); !errors.Is(err, ErrInvalidTenantSettings) {
		t.Fatalf("expected settings validation error, got %v", err)
	}
}

func TestTenantServiceLifecycleUsesContextBoundRepository(t *testing.T) {
	repository := &tenantRepositoryStub{}
	service := NewTenantService(repository)
	ctx := tenantcontext.With(context.Background(), tenantcontext.Scope{TenantID: "tenant-1", UserID: "user-1"})
	if _, err := service.Suspend(ctx); err != nil || repository.transition != "suspended" {
		t.Fatalf("suspend err=%v transition=%q", err, repository.transition)
	}
	if err := service.Delete(ctx); err != nil || repository.transition != "deleted" {
		t.Fatalf("delete err=%v transition=%q", err, repository.transition)
	}
}
