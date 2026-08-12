package config

import (
	"os"
	"path/filepath"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestRepositoryYAMLParses(t *testing.T) {
	for _, relative := range []string{filepath.Join("..", "..", "..", "..", "docker", "compose.yaml"), filepath.Join("..", "..", "..", "..", ".github", "workflows", "ci.yml")} {
		contents, err := os.ReadFile(relative)
		if err != nil {
			t.Fatalf("read %s: %v", relative, err)
		}
		var document any
		if err := yaml.Unmarshal(contents, &document); err != nil {
			t.Fatalf("parse %s: %v", relative, err)
		}
	}
}
