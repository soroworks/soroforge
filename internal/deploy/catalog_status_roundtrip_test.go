package deploy_test

import (
	"encoding/json"
	"testing"

	"github.com/soroworks/soroforge/internal/catalog"
	"github.com/soroworks/soroforge/internal/deploy"
	"github.com/stretchr/testify/require"
)

func TestCatalogStatusJSONRoundTrip(t *testing.T) {
	entry := &catalog.Result{
		URL:       "https://sorovault.example.com/contracts/abc123",
		Created:   true,
		Changed:   false,
		Functions: 5,
	}
	original := deploy.CatalogStatus{
		Registry: "https://sorovault.example.com",
		OK:       true,
		Error:    "",
		Entry:    entry,
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var got deploy.CatalogStatus
	require.NoError(t, json.Unmarshal(data, &got))

	require.Equal(t, original.Registry, got.Registry)
	require.Equal(t, original.OK, got.OK)
	require.Equal(t, original.Error, got.Error)
	require.NotNil(t, got.Entry)
	require.Equal(t, *original.Entry, *got.Entry)
}

func TestCatalogStatusJSONRoundTripError(t *testing.T) {
	original := deploy.CatalogStatus{
		Registry: "https://sorovault.example.com",
		OK:       false,
		Error:    "registration failed: 503 Service Unavailable",
		Entry:    nil,
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var got deploy.CatalogStatus
	require.NoError(t, json.Unmarshal(data, &got))

	require.Equal(t, original, got)
}
