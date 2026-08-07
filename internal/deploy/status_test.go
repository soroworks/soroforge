package deploy_test

import (
	"context"
	"testing"

	"github.com/soroworks/soroforge/internal/deploy"
	"github.com/soroworks/soroforge/internal/stellar"
	"github.com/soroworks/soroforge/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Drift detection is what makes the recorded history trustworthy: it tests
// SoroForge's claim against the ledger rather than assuming the database is
// right. These cases cover each way that comparison can come out.

func TestStatusInSync(t *testing.T) {
	h := newHarness(t, harnessOptions{})
	trackContract(t, h, h.wasmHash)

	result, err := h.svc.Status(context.Background(), deploy.StatusRequest{Alias: "counter"})
	require.NoError(t, err)

	assert.Equal(t, deploy.StateInSync, result.State)
	assert.True(t, result.InSync())
	assert.Equal(t, result.ExpectedWasmHash, result.ActualWasmHash)
	assert.Equal(t, h.contractID, result.ContractID)
}

func TestStatusDetectsDrift(t *testing.T) {
	// Someone upgraded the contract outside SoroForge. The database says one
	// thing, the chain says another, and the tool must report the chain.
	h := newHarness(t, harnessOptions{})
	ctx := context.Background()

	recorded := stellar.WasmHash([]byte("what soroforge deployed"))
	live := stellar.WasmHash([]byte("what is actually running"))

	_, err := h.store.UpsertContract(ctx, store.Contract{
		Alias: "counter", Network: "testnet", ContractID: h.contractID,
		CurrentWasmHash: stellar.HashHex(recorded),
	})
	require.NoError(t, err)
	require.NoError(t, h.client.SetContractInstance(h.contractID, live))

	result, err := h.svc.Status(ctx, deploy.StatusRequest{Alias: "counter"})
	require.NoError(t, err)

	assert.Equal(t, deploy.StateDrift, result.State)
	assert.False(t, result.InSync())
	assert.Equal(t, stellar.HashHex(recorded), result.ExpectedWasmHash)
	assert.Equal(t, stellar.HashHex(live), result.ActualWasmHash)
	assert.Contains(t, result.Detail, "outside SoroForge")
}

func TestStatusUntracked(t *testing.T) {
	// SoroForge has no record of this contract on this network.
	h := newHarness(t, harnessOptions{})

	result, err := h.svc.Status(context.Background(), deploy.StatusRequest{Alias: "counter"})
	require.NoError(t, err)

	assert.Equal(t, deploy.StateUntracked, result.State)
	assert.False(t, result.InSync())
	assert.Empty(t, result.ContractID)
	assert.Contains(t, result.Detail, "not tracked")
}

func TestStatusMissingOnChain(t *testing.T) {
	// SoroForge has a record, but the ledger has no such contract — it was
	// never deployed to this network, or its instance entry expired. That is a
	// different problem from drift and must not be reported as one.
	h := newHarness(t, harnessOptions{})
	ctx := context.Background()

	_, err := h.store.UpsertContract(ctx, store.Contract{
		Alias: "counter", Network: "testnet", ContractID: h.contractID,
		CurrentWasmHash: stellar.HashHex(h.wasmHash),
	})
	require.NoError(t, err)
	// Deliberately no ledger entry.

	result, err := h.svc.Status(ctx, deploy.StatusRequest{Alias: "counter"})
	require.NoError(t, err)

	assert.Equal(t, deploy.StateMissing, result.State)
	assert.False(t, result.InSync())
	assert.Contains(t, result.Detail, "no instance")
}

func TestStatusIsPerNetwork(t *testing.T) {
	// A contract in sync on testnet says nothing about mainnet.
	h := newHarness(t, harnessOptions{})
	trackContract(t, h, h.wasmHash)

	ctx := context.Background()

	testnet, err := h.svc.Status(ctx, deploy.StatusRequest{Alias: "counter", Network: "testnet"})
	require.NoError(t, err)
	assert.Equal(t, deploy.StateInSync, testnet.State)

	mainnet, err := h.svc.Status(ctx, deploy.StatusRequest{Alias: "counter", Network: "mainnet"})
	require.NoError(t, err)
	assert.Equal(t, deploy.StateUntracked, mainnet.State)
}

func TestStatusNeedsNoSigner(t *testing.T) {
	// Checking status is a read; requiring a signing key for it would mean
	// exposing a key to run a health check in CI.
	h := newHarness(t, harnessOptions{noSigner: true})
	trackContract(t, h, h.wasmHash)

	result, err := h.svc.Status(context.Background(), deploy.StatusRequest{Alias: "counter"})
	require.NoError(t, err)
	assert.Equal(t, deploy.StateInSync, result.State)
}

func TestStatusRejectsUnknownContract(t *testing.T) {
	h := newHarness(t, harnessOptions{})

	_, err := h.svc.Status(context.Background(), deploy.StatusRequest{Alias: "ghost"})
	assert.ErrorContains(t, err, "unknown contract")
}

func TestListReturnsTrackedContracts(t *testing.T) {
	h := newHarness(t, harnessOptions{})
	ctx := context.Background()

	for _, c := range []store.Contract{
		{Alias: "counter", Network: "testnet", ContractID: "C1", CurrentWasmHash: "a"},
		{Alias: "counter", Network: "mainnet", ContractID: "C2", CurrentWasmHash: "b"},
	} {
		_, err := h.store.UpsertContract(ctx, c)
		require.NoError(t, err)
	}

	all, err := h.svc.List(ctx, "")
	require.NoError(t, err)
	assert.Len(t, all, 2, "an empty network lists every network")

	testnet, err := h.svc.List(ctx, "testnet")
	require.NoError(t, err)
	require.Len(t, testnet, 1)
	assert.Equal(t, "C1", testnet[0].ContractID)
}

func TestListRejectsUnknownNetwork(t *testing.T) {
	h := newHarness(t, harnessOptions{})

	_, err := h.svc.List(context.Background(), "staging")
	assert.ErrorContains(t, err, "unknown network")
}

func TestHistoryReturnsFullTimeline(t *testing.T) {
	// The history is the product: a deploy followed by two upgrades should read
	// back as a version timeline, newest first.
	h := newHarness(t, harnessOptions{})
	ctx := context.Background()

	for _, d := range []store.Deployment{
		{
			Alias: "counter", Network: "testnet", ContractID: h.contractID, WasmHash: "v1",
			Action: store.ActionDeploy, DeployerPubkey: "GABC",
		},
		{
			Alias: "counter", Network: "testnet", ContractID: h.contractID, WasmHash: "v2",
			Action: store.ActionUpgrade, DeployerPubkey: "GABC",
		},
		{
			Alias: "counter", Network: "testnet", ContractID: h.contractID, WasmHash: "v3",
			Action: store.ActionUpgrade, DeployerPubkey: "GDEF",
		},
	} {
		_, err := h.store.RecordDeployment(ctx, d)
		require.NoError(t, err)
	}

	history, err := h.svc.History(ctx, "testnet", "counter")
	require.NoError(t, err)
	require.Len(t, history, 3)

	assert.Equal(t, "v3", history[0].WasmHash)
	assert.Equal(t, "GDEF", history[0].DeployerPubkey)
	assert.Equal(t, store.ActionDeploy, history[2].Action)
}

func TestHistoryDefaultsToConfiguredNetwork(t *testing.T) {
	h := newHarness(t, harnessOptions{})
	ctx := context.Background()

	_, err := h.store.RecordDeployment(ctx, store.Deployment{
		Alias: "counter", Network: "testnet", ContractID: h.contractID, WasmHash: "v1",
		Action: store.ActionDeploy, DeployerPubkey: "GABC",
	})
	require.NoError(t, err)

	history, err := h.svc.History(ctx, "", "counter")
	require.NoError(t, err)
	assert.Len(t, history, 1)
}

func TestNewServiceValidatesRequiredDependencies(t *testing.T) {
	_, err := deploy.New(deploy.Options{})
	assert.ErrorContains(t, err, "config is required")

	_, err = deploy.New(deploy.Options{Config: newHarness(t, harnessOptions{}).svc.Config()})
	assert.ErrorContains(t, err, "store is required")
}
