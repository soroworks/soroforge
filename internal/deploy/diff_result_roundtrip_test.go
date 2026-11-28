package deploy_test

import (
	"encoding/json"
	"testing"

	"github.com/soroworks/soroforge/internal/catalog"
	"github.com/soroworks/soroforge/internal/deploy"
	"github.com/stretchr/testify/require"
)

func TestDiffResultJSONRoundTrip(t *testing.T) {
	original := deploy.DiffResult{
		Alias:      "counter",
		Network:    "testnet",
		ContractID: "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABSC4",
		Registry:   "https://vault.example.org",
		From:       "deadbeef01",
		To:         "deadbeef02",
		Changes: []catalog.Change{
			{
				Kind:      catalog.Changed,
				Item:      "fn",
				Name:      "increment",
				Signature: "fn increment(by: u32) -> u32",
				Previous:  "fn increment() -> u32",
				Breaking:  true,
			},
			{
				Kind:      catalog.Added,
				Item:      "fn",
				Name:      "reset",
				Signature: "fn reset()",
				Breaking:  false,
			},
		},
		Breaking: true,
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))
	for _, key := range []string{"alias", "network", "contract_id", "registry", "from", "to", "changes", "breaking"} {
		require.Contains(t, raw, key)
	}

	var got deploy.DiffResult
	require.NoError(t, json.Unmarshal(data, &got))
	require.Equal(t, original, got)
}

func TestDiffResultJSONRoundTripEmptyChanges(t *testing.T) {
	original := deploy.DiffResult{
		Alias:      "counter",
		Network:    "testnet",
		ContractID: "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABSC4",
		Registry:   "https://vault.example.org",
		From:       "deadbeef02",
		To:         "deadbeef02",
		Changes:    []catalog.Change{},
		Breaking:   false,
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var got deploy.DiffResult
	require.NoError(t, json.Unmarshal(data, &got))
	require.NotNil(t, got.Changes)
	require.Equal(t, original, got)
}
