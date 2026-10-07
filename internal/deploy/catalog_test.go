package deploy_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/soroworks/soroforge/internal/catalog"
	"github.com/soroworks/soroforge/internal/deploy"
	"github.com/soroworks/soroforge/internal/stellar"
)

// fakeRegistrar records what it was asked to publish.
type fakeRegistrar struct {
	calls []string // baseURL + " " + contractID
	err   error
}

func (f *fakeRegistrar) Register(_ context.Context, baseURL, contractID string) (*catalog.Result, error) {
	f.calls = append(f.calls, baseURL+" "+contractID)
	if f.err != nil {
		return nil, f.err
	}
	return &catalog.Result{URL: baseURL + "/api/contracts/" + contractID, Created: true, Changed: true, Functions: 3}, nil
}

// catalogConfig gives testnet a registry and leaves mainnet without one.
const catalogConfig = `
version: 1
default_network: testnet
networks:
  testnet:
    rpc_url: https://rpc.example.org
    passphrase: "Test SDF Network ; September 2015"
    sorovault_url: http://vault.example.org
  mainnet:
    rpc_url: https://mainnet.example.org
    passphrase: "Public Global Stellar Network ; September 2015"
contracts:
  counter:
    wasm: ./counter.wasm
`

func TestDeployRegistersWithSoroVault(t *testing.T) {
	reg := &fakeRegistrar{}
	h := newHarness(t, harnessOptions{configBody: catalogConfig, catalog: reg})

	result, err := h.svc.Deploy(context.Background(), deploy.DeployRequest{Alias: "counter"})
	require.NoError(t, err)

	assert.Equal(t, []string{"http://vault.example.org " + h.contractID}, reg.calls)
	require.NotNil(t, result.Catalog)
	assert.True(t, result.Catalog.OK)
	assert.Equal(t, "http://vault.example.org", result.Catalog.Registry)
	assert.Equal(t, 3, result.Catalog.Entry.Functions)
}

func TestDeployCatalogFailureDoesNotFailTheDeploy(t *testing.T) {
	reg := &fakeRegistrar{err: errors.New("connection refused")}
	h := newHarness(t, harnessOptions{configBody: catalogConfig, catalog: reg})
	ctx := context.Background()

	result, err := h.svc.Deploy(ctx, deploy.DeployRequest{Alias: "counter"})
	require.NoError(t, err, "the contract is live; a registry outage is not a deploy failure")

	require.NotNil(t, result.Catalog)
	assert.False(t, result.Catalog.OK)
	assert.Contains(t, result.Catalog.Error, "connection refused")

	// The deploy is still recorded.
	tracked, err := h.store.GetContract(ctx, "testnet", "counter")
	require.NoError(t, err)
	assert.Equal(t, h.contractID, tracked.ContractID)
}

func TestDeployWithoutRegistryConfiguredSkipsCatalog(t *testing.T) {
	reg := &fakeRegistrar{}
	h := newHarness(t, harnessOptions{configBody: catalogConfig, catalog: reg})

	result, err := h.svc.Deploy(context.Background(), deploy.DeployRequest{Alias: "counter", Network: "mainnet"})
	require.NoError(t, err)
	assert.Nil(t, result.Catalog)
	assert.Empty(t, reg.calls)
}

func TestDeployDryRunDoesNotRegister(t *testing.T) {
	reg := &fakeRegistrar{}
	h := newHarness(t, harnessOptions{configBody: catalogConfig, catalog: reg})

	result, err := h.svc.Deploy(context.Background(), deploy.DeployRequest{Alias: "counter", DryRun: true})
	require.NoError(t, err)
	assert.Nil(t, result.Catalog)
	assert.Empty(t, reg.calls, "a contract that does not exist cannot be catalogued")
}

func TestUpgradeRegistersNewInterfaceVersion(t *testing.T) {
	reg := &fakeRegistrar{}
	h := newHarness(t, harnessOptions{configBody: catalogConfig, wasm: upgradedWasm, catalog: reg})
	trackContract(t, h, stellar.WasmHash([]byte(testWasm)))

	result, err := h.svc.Upgrade(context.Background(), deploy.UpgradeRequest{Alias: "counter"})
	require.NoError(t, err)

	assert.Equal(t, []string{"http://vault.example.org " + h.contractID}, reg.calls)
	require.NotNil(t, result.Catalog)
	assert.True(t, result.Catalog.OK)
}

func TestUpgradeWithNoChangeDoesNotRegister(t *testing.T) {
	reg := &fakeRegistrar{}
	h := newHarness(t, harnessOptions{configBody: catalogConfig, catalog: reg})
	trackContract(t, h, h.wasmHash)

	result, err := h.svc.Upgrade(context.Background(), deploy.UpgradeRequest{Alias: "counter"})
	require.NoError(t, err)
	require.True(t, result.NoChange)
	assert.Nil(t, result.Catalog)
	assert.Empty(t, reg.calls)
}
