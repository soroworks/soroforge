package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsSQLiteURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{"sqlite file path", "sqlite:///home/user/history.db", true},
		{"sqlite memory", "sqlite://:memory:", true},
		{"sqlite relative path", "sqlite://./history.db", true},
		{"sqlite upper case", "SQLITE:///home/user/history.db", true},
		{"sqlite mixed case", "Sqlite:///home/user/history.db", true},
		{"sqlite with whitespace", "  sqlite:///home/user/history.db  ", true},
		{"postgres url", "postgres://user:pass@localhost/db", false},
		{"postgres with sslmode", "postgres://user:pass@localhost/db?sslmode=disable", false},
		{"empty string", "", false},
		{"whitespace only", "   ", false},
		{"sqlite without prefix", "file:///home/user/history.db", false},
		{"just sqlite", "sqlite:", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsSQLiteURL(tt.url)
			assert.Equal(t, tt.expected, result)
		})
	}
}
