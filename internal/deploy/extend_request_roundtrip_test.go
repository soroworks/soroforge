package deploy_test

import (
	"encoding/json"
	"testing"

	"github.com/soroworks/soroforge/internal/deploy"
	"github.com/stretchr/testify/require"
)

func TestExtendRequestJSONRoundTrip(t *testing.T) {
	original := deploy.ExtendRequest{
		Alias:   "counter",
		Network: "testnet",
		Ledgers: 50000,
		Notes:   "monthly TTL renewal",
		DryRun:  true,
		Actor:   "cli",
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))
	for _, key := range []string{"alias", "network", "ledgers", "notes", "dry_run"} {
		require.Contains(t, raw, key)
	}
	require.NotContains(t, raw, "actor")
	require.NotContains(t, raw, "Actor")

	var got deploy.ExtendRequest
	require.NoError(t, json.Unmarshal(data, &got))

	expected := original
	expected.Actor = ""
	require.Equal(t, expected, got)
}

func TestExtendRequestJSONRoundTripOmitEmpty(t *testing.T) {
	original := deploy.ExtendRequest{
		Alias: "counter",
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))
	require.Contains(t, raw, "alias")
	for _, key := range []string{"network", "ledgers", "notes", "dry_run"} {
		require.NotContains(t, raw, key)
	}

	var got deploy.ExtendRequest
	require.NoError(t, json.Unmarshal(data, &got))
	require.Equal(t, original, got)
}
