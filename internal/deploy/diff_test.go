package deploy_test

import (
	"context"
	"errors"
	"testing"

	"github.com/soroworks/soroforge/internal/catalog"
	"github.com/soroworks/soroforge/internal/deploy"
	"github.com/soroworks/soroforge/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const diffConfig = `
version: 1
default_network: testnet
networks:
  testnet:
    rpc_url: https://rpc.example.org
    passphrase: "Test SDF Network ; September 2015"
    sorovault_url: https://vault.example.org
  bare:
    rpc_url: https://rpc.example.org
    passphrase: "Test SDF Network ; September 2015"
contracts:
  counter:
    wasm: ./counter.wasm
`

// fakeVault is a catalog that can both register and read.
type fakeVault struct {
	versions   []catalog.Version
	interfaces map[string]*catalog.Interface
	err        error
	baseURLs   []string
}

func (f *fakeVault) Register(context.Context, string, string) (*catalog.Result, error) {
	return &catalog.Result{}, nil
}

func (f *fakeVault) Versions(_ context.Context, baseURL, _ string) ([]catalog.Version, error) {
	f.baseURLs = append(f.baseURLs, baseURL)
	return f.versions, f.err
}

func (f *fakeVault) Interface(_ context.Context, _, _, wasmHash string) (*catalog.Interface, error) {
	if in, ok := f.interfaces[wasmHash]; ok {
		return in, nil
	}
	return nil, errors.New("sorovault: version not found")
}

func newDiffHarness(t *testing.T, vault catalog.Registrar) *harness {
	t.Helper()
	h := newHarness(t, harnessOptions{configBody: diffConfig, catalog: vault, noSigner: true})
	_, err := h.store.UpsertContract(context.Background(), store.Contract{
		Alias: "counter", Network: "testnet", ContractID: h.contractID, CurrentWasmHash: "cccc",
	})
	require.NoError(t, err)
	return h
}

func threeVersionVault() *fakeVault {
	inc := catalog.Function{Name: "increment", Outputs: []catalog.Type{{Display: "u32"}}}
	incBy := catalog.Function{
		Name:    "increment",
		Inputs:  []catalog.Param{{Name: "by", Type: catalog.Type{Display: "u32"}}},
		Outputs: []catalog.Type{{Display: "u32"}},
	}
	reset := catalog.Function{Name: "reset"}
	return &fakeVault{
		versions: []catalog.Version{{WasmHash: "cccc", Current: true}, {WasmHash: "bbbb"}, {WasmHash: "aaaa"}},
		interfaces: map[string]*catalog.Interface{
			"aaaa": {Functions: []catalog.Function{inc}},
			"bbbb": {Functions: []catalog.Function{inc, reset}},
			"cccc": {Functions: []catalog.Function{incBy, reset}},
		},
	}
}

func TestDiffDefaultsToLastTwoVersions(t *testing.T) {
	vault := threeVersionVault()
	h := newDiffHarness(t, vault)

	result, err := h.svc.Diff(context.Background(), deploy.DiffRequest{Alias: "counter"})
	require.NoError(t, err)

	assert.Equal(t, "bbbb", result.From)
	assert.Equal(t, "cccc", result.To)
	assert.Equal(t, h.contractID, result.ContractID)
	assert.Equal(t, "https://vault.example.org", result.Registry)
	assert.Equal(t, []string{"https://vault.example.org"}, vault.baseURLs)
	require.Len(t, result.Changes, 1)
	assert.Equal(t, catalog.Changed, result.Changes[0].Kind)
	assert.Equal(t, "fn increment() -> u32", result.Changes[0].Previous)
	assert.True(t, result.Breaking)
}

func TestDiffExplicitPair(t *testing.T) {
	h := newDiffHarness(t, threeVersionVault())

	result, err := h.svc.Diff(context.Background(), deploy.DiffRequest{Alias: "counter", From: "aaaa", To: "bbbb"})
	require.NoError(t, err)
	require.Len(t, result.Changes, 1)
	assert.Equal(t, catalog.Added, result.Changes[0].Kind)
	assert.False(t, result.Breaking)

	// --to alone compares against the version recorded before it.
	result, err = h.svc.Diff(context.Background(), deploy.DiffRequest{Alias: "counter", To: "bbbb"})
	require.NoError(t, err)
	assert.Equal(t, "aaaa", result.From)
}

func TestDiffIdenticalVersionsHasEmptyChanges(t *testing.T) {
	h := newDiffHarness(t, threeVersionVault())

	result, err := h.svc.Diff(context.Background(), deploy.DiffRequest{Alias: "counter", From: "cccc", To: "cccc"})
	require.NoError(t, err)
	require.NotNil(t, result.Changes)
	assert.Empty(t, result.Changes)
	assert.False(t, result.Breaking)
}

func TestDiffRequiresSoroVaultURL(t *testing.T) {
	h := newDiffHarness(t, threeVersionVault())

	_, err := h.svc.Diff(context.Background(), deploy.DiffRequest{Alias: "counter", Network: "bare"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "network bare has no sorovault_url")
}

func TestDiffErrors(t *testing.T) {
	t.Run("untracked", func(t *testing.T) {
		h := newHarness(t, harnessOptions{configBody: diffConfig, catalog: threeVersionVault(), noSigner: true})
		_, err := h.svc.Diff(context.Background(), deploy.DiffRequest{Alias: "counter"})
		assert.ErrorContains(t, err, "not tracked")
	})

	t.Run("only one version", func(t *testing.T) {
		vault := threeVersionVault()
		vault.versions = vault.versions[:1]
		h := newDiffHarness(t, vault)
		_, err := h.svc.Diff(context.Background(), deploy.DiffRequest{Alias: "counter"})
		assert.ErrorContains(t, err, "no version is recorded before cccc")
	})

	t.Run("unknown --to", func(t *testing.T) {
		h := newDiffHarness(t, threeVersionVault())
		_, err := h.svc.Diff(context.Background(), deploy.DiffRequest{Alias: "counter", To: "ffff"})
		assert.ErrorContains(t, err, "ffff is not recorded")
	})

	t.Run("registry error", func(t *testing.T) {
		vault := threeVersionVault()
		vault.err = errors.New("sorovault: unavailable")
		h := newDiffHarness(t, vault)
		_, err := h.svc.Diff(context.Background(), deploy.DiffRequest{Alias: "counter"})
		assert.ErrorContains(t, err, "sorovault: unavailable")
	})

	t.Run("missing interface", func(t *testing.T) {
		h := newDiffHarness(t, threeVersionVault())
		_, err := h.svc.Diff(context.Background(), deploy.DiffRequest{Alias: "counter", From: "zzzz", To: "cccc"})
		assert.ErrorContains(t, err, "read interface of counter at zzzz")
	})

	t.Run("catalog that cannot read", func(t *testing.T) {
		h := newHarness(t, harnessOptions{configBody: diffConfig, catalog: registrarOnly{}, noSigner: true})
		_, err := h.svc.Diff(context.Background(), deploy.DiffRequest{Alias: "counter"})
		assert.ErrorContains(t, err, "cannot read recorded interfaces")
	})
}

type registrarOnly struct{}

func (registrarOnly) Register(context.Context, string, string) (*catalog.Result, error) {
	return &catalog.Result{}, nil
}
