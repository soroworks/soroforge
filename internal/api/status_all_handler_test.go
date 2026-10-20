package api_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/soroworks/soroforge/internal/deploy"
	"github.com/soroworks/soroforge/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusAllUnknownNetworkMatchesPerAliasRoute(t *testing.T) {
	svc := &fakeService{err: fmt.Errorf(`unknown network "nowhere"`)}
	router := newTestRouter(t, svc)

	all := do(t, router, http.MethodGet, "/v1/contracts/nowhere/status", nil, nil)
	one := do(t, router, http.MethodGet, "/v1/contracts/nowhere/counter/status", nil, nil)

	assert.Equal(t, http.StatusBadRequest, all.Code)
	assert.Equal(t, one.Code, all.Code, "both status routes must agree on an unknown network")
	assert.Equal(t, "nowhere", svc.lastNetwork)

	var body map[string]string
	require.NoError(t, json.Unmarshal(all.Body.Bytes(), &body))
	assert.Contains(t, body["error"], "unknown network")
}

func TestStatusAllMapsServiceErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"not found", fmt.Errorf("lookup: %w", store.ErrNotFound), http.StatusNotFound},
		{"client error", errors.New("contract counter is not tracked"), http.StatusBadRequest},
		{"server error", errors.New("connect to testnet: dial tcp: connection refused"), http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := newTestRouter(t, &fakeService{err: tt.err})
			rec := do(t, router, http.MethodGet, "/v1/contracts/testnet/status", nil, nil)
			assert.Equal(t, tt.want, rec.Code)

			var body map[string]string
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			assert.Equal(t, tt.err.Error(), body["error"])
		})
	}
}

func TestStatusAllInSyncNetwork(t *testing.T) {
	svc := &fakeService{networkStatus: &deploy.NetworkStatus{
		Network:   "testnet",
		InSync:    true,
		Contracts: []deploy.StatusResult{},
	}}
	router := newTestRouter(t, svc)

	rec := do(t, router, http.MethodGet, "/v1/contracts/testnet/status", nil, nil)
	require.Equal(t, http.StatusOK, rec.Code)

	var got deploy.NetworkStatus
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "testnet", got.Network)
	assert.True(t, got.InSync)
	require.NotNil(t, got.Contracts, "contracts must be an empty array, not null")
	assert.Empty(t, got.Contracts)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &raw))
	assert.JSONEq(t, `[]`, string(raw["contracts"]))
}
