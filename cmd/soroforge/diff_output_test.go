package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/soroworks/soroforge/internal/catalog"
	"github.com/soroworks/soroforge/internal/deploy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleDiff() *deploy.DiffResult {
	return &deploy.DiffResult{
		Alias: "counter", Network: "testnet",
		From: "3f2a0000aaaa", To: "9c1e0000bbbb",
		Changes: []catalog.Change{
			{Kind: catalog.Added, Item: "fn", Name: "reset", Signature: "fn reset(admin: Address)"},
			{Kind: catalog.Changed, Item: "fn", Name: "increment", Signature: "fn increment(by: u32) -> u32",
				Previous: "fn increment() -> u32", Breaking: true},
			{Kind: catalog.Removed, Item: "struct", Name: "LegacyConfig",
				Signature: "struct LegacyConfig { admin: Address }", Breaking: true},
		},
		Breaking: true,
	}
}

func TestPrintDiffText(t *testing.T) {
	saved := flags
	t.Cleanup(func() { flags = saved })
	flags.jsonOutput = false

	got := captureStdout(t, func() error { return printDiff(sampleDiff()) })
	lines := strings.Split(got, "\n")
	assert.Equal(t, "counter on testnet: 3f2a0000aaaa  →  9c1e0000bbbb", lines[0])
	assert.Contains(t, got, "+ fn reset(admin: Address)")
	assert.Contains(t, got, "~ fn increment(by: u32) -> u32")
	assert.Contains(t, got, "(was: fn increment() -> u32) [breaking]")
	assert.Contains(t, got, "- struct LegacyConfig")
	assert.Contains(t, got, "breaking changes")
}

func TestPrintDiffNoChanges(t *testing.T) {
	saved := flags
	t.Cleanup(func() { flags = saved })
	flags.jsonOutput = false

	got := captureStdout(t, func() error {
		return printDiff(&deploy.DiffResult{Alias: "counter", Network: "testnet", From: "a", To: "b", Changes: []catalog.Change{}})
	})
	assert.Contains(t, got, "No interface changes.")
}

func TestPrintDiffJSON(t *testing.T) {
	saved := flags
	t.Cleanup(func() { flags = saved })
	flags.jsonOutput = true

	got := captureStdout(t, func() error { return printDiff(sampleDiff()) })
	var decoded deploy.DiffResult
	require.NoError(t, json.Unmarshal([]byte(got), &decoded))
	assert.True(t, decoded.Breaking)
	assert.Len(t, decoded.Changes, 3)
	assert.Equal(t, "9c1e0000bbbb", decoded.To)
}
