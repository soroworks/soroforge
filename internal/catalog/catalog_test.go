package catalog_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/soroworks/soroforge/internal/catalog"
)

const contractID = "CCLV77FYLRJMBGTILTKVIM76SI6JY56H7KTT47HE26UF4F265UYRDSR4"

func TestRegister(t *testing.T) {
	t.Parallel()

	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/contracts", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))

		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{
		  "contract": {"contract_id": "` + contractID + `", "network": "testnet"},
		  "interface": {"functions": [{"name": "increment"}, {"name": "get"}]},
		  "created": true, "changed": true
		}`))
	}))
	defer srv.Close()

	got, err := catalog.NewSoroVault(nil).Register(context.Background(), srv.URL+"/", contractID)
	require.NoError(t, err)

	assert.Equal(t, map[string]any{"contract_id": contractID}, gotBody)
	assert.Equal(t, &catalog.Result{
		URL:       srv.URL + "/api/contracts/" + contractID + "?network=testnet",
		Created:   true,
		Changed:   true,
		Functions: 2,
	}, got)
}

func TestRegisterAlreadyKnownIs200(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"contract": {"network": "testnet"}, "interface": {"functions": []}}`))
	}))
	defer srv.Close()

	got, err := catalog.NewSoroVault(nil).Register(context.Background(), srv.URL, contractID)
	require.NoError(t, err)
	assert.False(t, got.Created)
	assert.False(t, got.Changed)
}

func TestRegisterSurfacesRegistryErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		status   int
		body     string
		contains string
	}{
		{"json error body", http.StatusNotFound, `{"error":"stellar: contract not found"}`, "contract not found"},
		{"no interface", http.StatusUnprocessableEntity, `{"error":"spec: no contractspecv0 section"}`, "contractspecv0"},
		{"opaque body", http.StatusBadGateway, `<html>`, "502"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			_, err := catalog.NewSoroVault(nil).Register(context.Background(), srv.URL, contractID)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.contains)
		})
	}
}

func TestRegisterUnreachable(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()

	_, err := catalog.NewSoroVault(nil).Register(context.Background(), url, contractID)
	assert.ErrorContains(t, err, "sorovault")
}
