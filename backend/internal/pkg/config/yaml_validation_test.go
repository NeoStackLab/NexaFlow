package config

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"go.yaml.in/yaml/v3"
)

func TestRepositoryYAMLParses(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..")
	paths := []string{
		filepath.Join(root, "backend", "configs", "config.production.yaml"),
		filepath.Join("..", "..", "..", "..", "docker", "compose.yaml"),
		filepath.Join(root, "docker", "compose.production.yaml"),
		filepath.Join("..", "..", "..", "..", "docker", "observability", "otel-collector.yaml"),
		filepath.Join("..", "..", "..", "..", "docker", "observability", "prometheus.yaml"),
		filepath.Join("..", "..", "..", "..", "docker", "observability", "alerts.yaml"),
	}
	workflowPaths, err := filepath.Glob(filepath.Join(root, ".github", "workflows", "*.yml"))
	if err != nil {
		t.Fatal(err)
	}
	paths = append(paths, workflowPaths...)
	kubernetesPaths, err := filepath.Glob(filepath.Join(root, "deploy", "kubernetes", "*.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	paths = append(paths, kubernetesPaths...)
	for _, relative := range paths {
		contents, err := os.ReadFile(relative)
		if err != nil {
			t.Fatalf("read %s: %v", relative, err)
		}
		decoder := yaml.NewDecoder(bytes.NewReader(contents))
		for documentNumber := 1; ; documentNumber++ {
			var document any
			if err := decoder.Decode(&document); err != nil {
				if err == io.EOF {
					break
				}
				t.Fatalf("parse %s document %d: %v", relative, documentNumber, err)
			}
		}
	}
}

func TestProductionConfigDoesNotContainDevelopmentSecrets(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..")
	contents, err := os.ReadFile(filepath.Join(root, "backend", "configs", "config.production.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		"change-this-development-secret-now",
		"nexaflow_dev",
		"nexaflow_admin_dev",
		"nexaflow_app_dev",
	} {
		if bytes.Contains(contents, []byte(forbidden)) {
			t.Fatalf("production config contains development secret %q", forbidden)
		}
	}
}

func TestProductionRejectsDevelopmentConfig(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	root := filepath.Join("..", "..", "..", "..")
	_, err := Load(filepath.Join(root, "backend", "configs", "config.yaml"))
	if err == nil || !strings.Contains(err.Error(), "must not load config.yaml") {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestSecretFilesOverrideConfigurationWithoutEnvironmentValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jwt-secret")
	if err := os.WriteFile(path, []byte("a-secret-loaded-from-a-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AUTH_JWT_SECRET_FILE", path)
	v := viper.New()
	if err := loadSecretFiles(v); err != nil {
		t.Fatal(err)
	}
	if got := v.GetString("auth.jwt_secret"); got != "a-secret-loaded-from-a-file" {
		t.Fatalf("secret = %q", got)
	}
}

func TestSecretFilesRejectEmptyFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("REDIS_PASSWORD_FILE", path)
	if err := loadSecretFiles(viper.New()); err == nil {
		t.Fatal("loadSecretFiles() accepted an empty secret")
	}
}

func TestSecretFilesPreserveSpacesAndRejectMultipleLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "database-password")
	if err := os.WriteFile(path, []byte(" leading-and-trailing-space \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DATABASE_PASSWORD_FILE", path)
	v := viper.New()
	if err := loadSecretFiles(v); err != nil {
		t.Fatal(err)
	}
	if got := v.GetString("database.password"); got != " leading-and-trailing-space " {
		t.Fatalf("secret = %q", got)
	}
	if err := os.WriteFile(path, []byte("first\nsecond\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := loadSecretFiles(viper.New()); err == nil {
		t.Fatal("loadSecretFiles() accepted a multi-line secret")
	}
}

func TestStringListAcceptsEnvironmentCSVAndTrimsValues(t *testing.T) {
	got := stringList(" hooks.example.com, ,api.example.com ")
	if len(got) != 2 || got[0] != "hooks.example.com" || got[1] != "api.example.com" {
		t.Fatalf("stringList() = %#v", got)
	}
}
