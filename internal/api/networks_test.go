package api_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/soroworks/soroforge/internal/api"
	"github.com/soroworks/soroforge/internal/config"
	"github.com/soroworks/soroforge/internal/deploy"
	"github.com/soroworks/soroforge/internal/netcheck"
	"github.com/soroworks/soroforge/internal/stellar"
	"github.com/soroworks/soroforge/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNetworksReportsHealthCachesResultsAndAllowsReadOnlyTokens(t *testing.T) {
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)

	svc := &fakeService{
		networkResults: []netcheck.Result{
			{
				Network:            "testnet",
				RedactedURL:        "https://rpc.example.org/soroban",
				Responded:          true,
				Latency:            18 * time.Millisecond,
				LatestLedger:       420001,
				ReportedPassphrase: "Test SDF Network ; September 2015",
				PassphraseVerified: true,
				PassphraseMatch:    true,
				OK:                 true,
			},
		},
	}

	router, err := api.NewRouter(api.Options{
		Service: svc,
		Tokens: []api.Token{
			api.NewToken("ci", testToken, false),
			api.NewToken("dashboard", testReadOnlyToken, true),
		},
		Readiness:        healthyReadiness{},
		Log:              slog.New(slog.NewTextHandler(io.Discard, nil)),
		NetworksCacheTTL: 5 * time.Second,
		Now:              func() time.Time { return now },
	})
	require.NoError(t, err)

	roTok := testReadOnlyToken
	rec := do(t, router, http.MethodGet, "/v1/networks", nil, &roTok)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var report api.NetworksResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &report))
	assert.True(t, report.AllOK)
	require.Len(t, report.Networks, 1)
	assert.Equal(t, "testnet", report.Networks[0].Network)
	assert.Equal(t, uint32(420001), report.Networks[0].LatestLedger)
	assert.True(t, report.Networks[0].PassphraseMatch)
	assert.Equal(t, 1, svc.networkChecks)

	// A repeated poll inside the TTL reuses the cached report without probing
	// the RPC endpoints again.
	now = now.Add(2 * time.Second)
	rec2 := do(t, router, http.MethodGet, "/v1/networks", nil, &roTok)
	require.Equal(t, http.StatusOK, rec2.Code)
	assert.Equal(t, 1, svc.networkChecks, "cached report must be served within NetworksCacheTTL")

	// Advancing past the TTL triggers a fresh probe and reports a degraded
	// network as a 200 with all_ok false, not a server error.
	svc.networkResults = []netcheck.Result{
		{
			Network:            "testnet",
			RedactedURL:        "https://rpc.example.org/soroban",
			Responded:          true,
			LatestLedger:       420009,
			ReportedPassphrase: "Wrong Passphrase",
			PassphraseVerified: true,
			PassphraseMatch:    false,
			Error:              "passphrase mismatch",
			OK:                 false,
		},
	}
	now = now.Add(4 * time.Second)
	rec3 := do(t, router, http.MethodGet, "/v1/networks", nil, &roTok)
	require.Equal(t, http.StatusOK, rec3.Code)
	require.NoError(t, json.Unmarshal(rec3.Body.Bytes(), &report))
	assert.False(t, report.AllOK)
	assert.Equal(t, uint32(420009), report.Networks[0].LatestLedger)
	assert.Equal(t, 2, svc.networkChecks)
}

func TestNetworksEmptyResultIsAnArrayNotNull(t *testing.T) {
	router := newTestRouter(t, &fakeService{})

	rec := do(t, router, http.MethodGet, "/v1/networks", nil, nil)
	require.Equal(t, http.StatusOK, rec.Code)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &raw))
	assert.JSONEq(t, `[]`, string(raw["networks"]))
	assert.JSONEq(t, `true`, string(raw["all_ok"]))
}

func TestDeployServiceCheckNetworksRedactsCredentialsEndToEnd(t *testing.T) {
	dir := t.TempDir()
	wasmPath := filepath.Join(dir, "counter.wasm")
	require.NoError(t, os.WriteFile(wasmPath, []byte("\x00asm\x01\x00\x00\x00"), 0o600))

	const secret = "super-secret-rpc-api-key"
	cfgBody := `
version: 1
default_network: testnet
networks:
  testnet:
    rpc_url: https://rpcuser:` + secret + `@rpc.example.org/soroban?api_key=` + secret + `
    passphrase: "Test SDF Network ; September 2015"
contracts:
  counter:
    wasm: ./counter.wasm
`
	cfgPath := filepath.Join(dir, "soroforge.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfgBody), 0o600))
	cfg, err := config.Load(cfgPath)
	require.NoError(t, err)

	mockClient := stellar.NewMockClient()
	mockClient.LedgerSeq = 9001
	mockClient.Passphrase = "Test SDF Network ; September 2015"

	realSvc, err := deploy.New(deploy.Options{
		Config:  cfg,
		Store:   store.NewMemoryStore(),
		Clients: func(string, config.Network) (stellar.Client, error) { return mockClient, nil },
		Log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	require.NoError(t, err)

	router := newTestRouter(t, realSvc)
	rec := do(t, router, http.MethodGet, "/v1/networks", nil, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	assert.NotContains(t, rec.Body.String(), secret, "RPC credentials must never appear in GET /v1/networks")
	assert.Contains(t, rec.Body.String(), `"rpc_url":"https://rpc.example.org/soroban"`)
	assert.Contains(t, rec.Body.String(), `"all_ok":true`)

	// The OpenAPI document and the router are kept in sync by the route tests,
	// but the new path must also be discoverable to API clients.
	empty := ""
	oaRec := do(t, router, http.MethodGet, "/openapi.json", nil, &empty)
	require.Equal(t, http.StatusOK, oaRec.Code)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(oaRec.Body.Bytes(), &doc))
	paths := doc["paths"].(map[string]any)
	assert.Contains(t, paths, "/v1/networks")
}
