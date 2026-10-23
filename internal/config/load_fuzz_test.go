package config

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// rawConfigLiteral matches the raw-string YAML configs written inline in
// config_test.go, so the seed corpus follows that file as it grows.
var rawConfigLiteral = regexp.MustCompile("(?s)`(\\s*version:.*?)`")

// FuzzLoad feeds arbitrary bytes to Load. Validation reports every problem at
// once and so walks partially valid structures; it must never panic on them.
func FuzzLoad(f *testing.F) {
	if repo, err := os.ReadFile(filepath.Join("..", "..", "soroforge.yaml")); err == nil {
		f.Add(repo)
	}
	if tests, err := os.ReadFile("config_test.go"); err == nil {
		for _, m := range rawConfigLiteral.FindAllSubmatch(tests, -1) {
			f.Add(m[1])
		}
	}
	f.Add([]byte(validConfig))
	f.Add([]byte(""))
	f.Add([]byte("version: 1\ncontracts:\n  c:\n"))
	f.Add([]byte("networks: [1, 2]\ncontracts: {a: null}\n"))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Each input lives in its own temp directory; wasm paths in the
		// corpus are relative, so they resolve inside it.
		path := filepath.Join(t.TempDir(), "soroforge.yaml")
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		_, _ = Load(path)
	})
}
