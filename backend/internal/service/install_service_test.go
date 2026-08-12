package service

import (
	"context"
	"errors"
	"testing"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
)

type installRepositoryStub struct {
	installed    bool
	completeHash string
	completeCall int
}

func (s *installRepositoryStub) Status(version string) model.InstallStatus {
	return model.InstallStatus{Installed: s.installed, Version: version}
}
func (s *installRepositoryStub) Environment(context.Context) []model.EnvironmentCheck { return nil }
func (s *installRepositoryStub) Complete(_ context.Context, input model.CompleteInstallationInput, hash string) (model.InstallationResult, error) {
	s.completeCall++
	s.completeHash = hash
	return model.InstallationResult{Username: input.Admin.Username}, nil
}
func (s *installRepositoryStub) RuntimeConfig() (model.InstallRuntimeConfig, error) {
	return model.InstallRuntimeConfig{}, errors.New("not configured")
}
func (s *installRepositoryStub) MigrateInstalled(context.Context) error { return nil }

func validInstallationInput() model.CompleteInstallationInput {
	return model.CompleteInstallationInput{
		Admin:   model.AdminInput{Username: "admin", Email: "admin@example.com", Password: "correct-horse-battery"},
		Company: model.CompanyInput{Name: "NexaFlow Labs", Industry: "manufacturing", DefaultLanguage: "zh-CN", Timezone: "Asia/Shanghai"},
	}
}

func TestInstallServiceRejectsAlreadyInstalledInstance(t *testing.T) {
	repository := &installRepositoryStub{installed: true}
	installService := NewInstallService(repository, "test")

	_, err := installService.Complete(context.Background(), validInstallationInput())
	if !errors.Is(err, ErrAlreadyInstalled) {
		t.Fatalf("Complete() error = %v, want ErrAlreadyInstalled", err)
	}
	if repository.completeCall != 0 {
		t.Fatalf("repository Complete() called %d times, want 0", repository.completeCall)
	}
}

func TestInstallServiceValidatesInputBeforeRepository(t *testing.T) {
	repository := &installRepositoryStub{}
	installService := NewInstallService(repository, "test")
	input := validInstallationInput()
	input.Admin.Password = "short"

	_, err := installService.Complete(context.Background(), input)
	if !errors.Is(err, ErrInvalidInstall) {
		t.Fatalf("Complete() error = %v, want ErrInvalidInstall", err)
	}
	if repository.completeCall != 0 {
		t.Fatalf("repository Complete() called %d times, want 0", repository.completeCall)
	}
}

func TestInstallServiceHashesAdministratorPassword(t *testing.T) {
	repository := &installRepositoryStub{}
	installService := NewInstallService(repository, "test")
	input := validInstallationInput()

	result, err := installService.Complete(context.Background(), input)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if result.Username != input.Admin.Username {
		t.Fatalf("username = %q, want %q", result.Username, input.Admin.Username)
	}
	if repository.completeHash == "" || repository.completeHash == input.Admin.Password {
		t.Fatal("administrator password was not hashed")
	}
	if repository.completeCall != 1 {
		t.Fatalf("repository Complete() called %d times, want 1", repository.completeCall)
	}
}
