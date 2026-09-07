package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	"github.com/NeoStackLab/NexaFlow/backend/internal/repository"
	"github.com/NeoStackLab/NexaFlow/backend/internal/tenantcontext"
)

var ErrInvalidTenantSettings = errors.New("invalid tenant settings")

var (
	localePattern = regexp.MustCompile(`^[a-z]{2}(?:-[A-Z]{2})?$`)
	domainPattern = regexp.MustCompile(`^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$`)
	colorPattern  = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
)

type TenantService interface {
	Get(context.Context) (model.TenantView, error)
	Update(context.Context, model.UpdateTenantInput) (model.TenantView, error)
	Suspend(context.Context) (model.TenantView, error)
	Archive(context.Context) (model.TenantView, error)
	Delete(context.Context) error
	Settings(context.Context) (model.TenantSettingsView, error)
	UpdateSettings(context.Context, model.UpdateTenantSettingsInput) (model.TenantSettingsView, error)
}

type tenantService struct{ repository repository.TenantRepository }

func NewTenantService(repository repository.TenantRepository) TenantService {
	return &tenantService{repository: repository}
}

func (s *tenantService) Get(ctx context.Context) (model.TenantView, error) {
	if _, err := tenantcontext.Require(ctx); err != nil {
		return model.TenantView{}, err
	}
	return s.repository.Get(ctx)
}

func (s *tenantService) Update(ctx context.Context, input model.UpdateTenantInput) (model.TenantView, error) {
	if _, err := tenantcontext.Require(ctx); err != nil {
		return model.TenantView{}, err
	}
	input.Name = strings.TrimSpace(input.Name)
	if len(input.Name) < 2 || len(input.Name) > 180 || input.ExpectedVersion < 1 {
		return model.TenantView{}, fmt.Errorf("%w: name must be 2-180 characters and expected_version is required", ErrInvalidTenantSettings)
	}
	return s.repository.Update(ctx, input)
}

func (s *tenantService) Suspend(ctx context.Context) (model.TenantView, error) {
	return s.transition(ctx, "suspended")
}

func (s *tenantService) Archive(ctx context.Context) (model.TenantView, error) {
	return s.transition(ctx, "archived")
}

func (s *tenantService) Delete(ctx context.Context) error {
	_, err := s.transition(ctx, "deleted")
	return err
}

func (s *tenantService) transition(ctx context.Context, target string) (model.TenantView, error) {
	if _, err := tenantcontext.Require(ctx); err != nil {
		return model.TenantView{}, err
	}
	return s.repository.Transition(ctx, target)
}

func (s *tenantService) Settings(ctx context.Context) (model.TenantSettingsView, error) {
	if _, err := tenantcontext.Require(ctx); err != nil {
		return model.TenantSettingsView{}, err
	}
	return s.repository.Settings(ctx)
}

func (s *tenantService) UpdateSettings(ctx context.Context, input model.UpdateTenantSettingsInput) (model.TenantSettingsView, error) {
	if _, err := tenantcontext.Require(ctx); err != nil {
		return model.TenantSettingsView{}, err
	}
	input.Timezone = strings.TrimSpace(input.Timezone)
	input.Locale = strings.TrimSpace(input.Locale)
	input.CustomDomain = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(input.CustomDomain), "."))
	input.Branding.LogoURL = strings.TrimSpace(input.Branding.LogoURL)
	for index, domain := range input.Security.AllowedEmailDomains {
		input.Security.AllowedEmailDomains[index] = strings.ToLower(strings.TrimSpace(domain))
	}
	for index, value := range input.Security.IPAllowlist {
		input.Security.IPAllowlist[index] = strings.TrimSpace(value)
	}
	if err := validateTenantSettings(input); err != nil {
		return model.TenantSettingsView{}, err
	}
	return s.repository.UpdateSettings(ctx, input)
}

func validateTenantSettings(input model.UpdateTenantSettingsInput) error {
	if input.ExpectedVersion < 1 {
		return fmt.Errorf("%w: expected_version is required", ErrInvalidTenantSettings)
	}
	if _, err := time.LoadLocation(input.Timezone); err != nil {
		return fmt.Errorf("%w: timezone is not an IANA timezone", ErrInvalidTenantSettings)
	}
	if !localePattern.MatchString(input.Locale) {
		return fmt.Errorf("%w: locale must use ll or ll-CC format", ErrInvalidTenantSettings)
	}
	if input.CustomDomain != "" && (net.ParseIP(input.CustomDomain) != nil || !domainPattern.MatchString(input.CustomDomain)) {
		return fmt.Errorf("%w: custom_domain must be a DNS hostname", ErrInvalidTenantSettings)
	}
	if input.Branding.PrimaryColor != "" && !colorPattern.MatchString(input.Branding.PrimaryColor) {
		return fmt.Errorf("%w: branding.primary_color must be a six-digit hex color", ErrInvalidTenantSettings)
	}
	if input.Branding.LogoURL != "" {
		parsed, err := url.ParseRequestURI(input.Branding.LogoURL)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			return fmt.Errorf("%w: branding.logo_url must be an HTTPS URL", ErrInvalidTenantSettings)
		}
	}
	security := input.Security
	if security.MinimumPasswordLength < 12 || security.MinimumPasswordLength > 128 || security.SessionTTLMinutes < 15 || security.SessionTTLMinutes > 43200 {
		return fmt.Errorf("%w: invalid password or session policy", ErrInvalidTenantSettings)
	}
	for _, domain := range security.AllowedEmailDomains {
		if _, err := mail.ParseAddress("security@" + domain); err != nil || !domainPattern.MatchString(domain) {
			return fmt.Errorf("%w: invalid allowed email domain", ErrInvalidTenantSettings)
		}
	}
	for _, value := range security.IPAllowlist {
		if net.ParseIP(value) == nil {
			if _, _, err := net.ParseCIDR(value); err != nil {
				return fmt.Errorf("%w: invalid IP allowlist entry", ErrInvalidTenantSettings)
			}
		}
	}
	return nil
}
