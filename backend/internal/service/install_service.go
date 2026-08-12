package service

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/config"
	"github.com/NeoStackLab/NexaFlow/backend/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrAlreadyInstalled = errors.New("NexaFlow is already installed")
	ErrInvalidInstall   = errors.New("invalid installation input")
)

type InstallService interface {
	Status() model.InstallStatus
	Environment(ctx context.Context) []model.EnvironmentCheck
	Readiness(ctx context.Context) model.InstallReadiness
	Complete(ctx context.Context, input model.CompleteInstallationInput) (model.InstallationResult, error)
}

type installService struct {
	repository       repository.InstallRepository
	version          string
	capabilityConfig *config.Config
}

func NewInstallService(repository repository.InstallRepository, version string, configs ...*config.Config) InstallService {
	var capabilityConfig *config.Config
	if len(configs) > 0 {
		capabilityConfig = configs[0]
	}
	return &installService{repository: repository, version: version, capabilityConfig: capabilityConfig}
}

func (s *installService) Status() model.InstallStatus { return s.repository.Status(s.version) }

func (s *installService) Environment(ctx context.Context) []model.EnvironmentCheck {
	return s.repository.Environment(ctx)
}

func (s *installService) Readiness(ctx context.Context) model.InstallReadiness {
	result := model.InstallReadiness{Infrastructure: s.repository.Environment(ctx), Capabilities: []model.CapabilityCheck{}}
	if s.capabilityConfig == nil {
		return result
	}
	cfg := s.capabilityConfig
	result.Capabilities = []model.CapabilityCheck{
		{ID: "ai", Name: "AI Provider", Configured: cfg.AI.APIKey != "", Message: "OpenAI-compatible chat and embeddings"},
		{ID: "billing", Name: "Stripe Billing", Configured: cfg.Billing.StripeSecretKey != "" && cfg.Billing.StripeWebhookSecret != "" && cfg.Billing.StripeProPriceID != "", Message: "Checkout, verified webhooks, and paid plans"},
		{ID: "storage", Name: "Object Storage", Configured: cfg.Storage.Provider == "local" || (cfg.Storage.Bucket != "" && cfg.Storage.AccessKeyID != "" && cfg.Storage.SecretAccessKey != ""), Message: strings.ToUpper(cfg.Storage.Provider) + " file storage"},
	}
	return result
}

func (s *installService) Complete(ctx context.Context, input model.CompleteInstallationInput) (model.InstallationResult, error) {
	if s.Status().Installed {
		return model.InstallationResult{}, ErrAlreadyInstalled
	}
	if err := validateCompleteInput(input); err != nil {
		return model.InstallationResult{}, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Admin.Password), 12)
	if err != nil {
		return model.InstallationResult{}, fmt.Errorf("hash administrator password: %w", err)
	}
	return s.repository.Complete(ctx, input, string(hash))
}

func validateCompleteInput(input model.CompleteInstallationInput) error {
	username := strings.TrimSpace(input.Admin.Username)
	if len(username) < 3 || len(username) > 64 {
		return fmt.Errorf("%w: administrator username must be 3-64 characters", ErrInvalidInstall)
	}
	if _, err := mail.ParseAddress(strings.TrimSpace(input.Admin.Email)); err != nil {
		return fmt.Errorf("%w: administrator email is invalid", ErrInvalidInstall)
	}
	if len(input.Admin.Password) < 12 || len(input.Admin.Password) > 72 {
		return fmt.Errorf("%w: administrator password must be 12-72 characters", ErrInvalidInstall)
	}
	if strings.TrimSpace(input.Company.Name) == "" {
		return fmt.Errorf("%w: company name is required", ErrInvalidInstall)
	}
	if !oneOf(input.Company.Industry, "manufacturing", "ecommerce", "healthcare", "logistics", "education", "other") {
		return fmt.Errorf("%w: unsupported industry", ErrInvalidInstall)
	}
	if !oneOf(input.Company.DefaultLanguage, "zh-CN", "en") {
		return fmt.Errorf("%w: unsupported language", ErrInvalidInstall)
	}
	if _, err := time.LoadLocation(input.Company.Timezone); err != nil {
		return fmt.Errorf("%w: invalid timezone", ErrInvalidInstall)
	}
	return nil
}

func oneOf(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}
