package deploy

import (
	"strings"
	"testing"

	"github.com/soroworks/soroforge/internal/config"
)

func TestCheckRequireNotes(t *testing.T) {
	s := &Service{}

	t.Run("disabled on network", func(t *testing.T) {
		r := resolved{
			NetworkName: "testnet",
			Network:     config.Network{RequireNotes: false},
		}
		if err := s.checkRequireNotes(r, "", false); err != nil {
			t.Fatalf("expected nil when require_notes is false, got %v", err)
		}
	})

	t.Run("required and missing on mutating run", func(t *testing.T) {
		r := resolved{
			NetworkName: "mainnet",
			Network:     config.Network{RequireNotes: true},
		}
		err := s.checkRequireNotes(r, "", false)
		if err == nil {
			t.Fatal("expected error when notes are empty on protected network, got nil")
		}
		if !strings.Contains(err.Error(), "requires deploy notes") {
			t.Errorf("error %q should explain the setting", err.Error())
		}
		if !strings.Contains(err.Error(), "e.g. --notes") {
			t.Errorf("error %q should give an example", err.Error())
		}
	})

	t.Run("required but provided", func(t *testing.T) {
		r := resolved{
			NetworkName: "mainnet",
			Network:     config.Network{RequireNotes: true},
		}
		if err := s.checkRequireNotes(r, "OPS-1234: routine upgrade", false); err != nil {
			t.Fatalf("expected nil when notes are provided, got %v", err)
		}
	})

	t.Run("required but exempt on dry run", func(t *testing.T) {
		r := resolved{
			NetworkName: "mainnet",
			Network:     config.Network{RequireNotes: true},
		}
		if err := s.checkRequireNotes(r, "", true); err != nil {
			t.Fatalf("expected nil on dry run, got %v", err)
		}
	})
}
