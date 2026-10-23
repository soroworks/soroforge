package deploy_test

import (
	"context"
	"os"
	"testing"

	"github.com/soroworks/soroforge/internal/deploy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Each case replaces the harness's built artifact and checks that Deploy fails
// on it before any RPC call is made.

func wasmPathOf(t *testing.T, h *harness) string {
	t.Helper()
	c, ok := h.cfg.Contracts["counter"]
	require.True(t, ok)
	return h.cfg.WasmPath(c)
}

func assertNoRPC(t *testing.T, h *harness) {
	t.Helper()
	assert.Zero(t, h.client.SimulateCount(), "nothing should be simulated")
	assert.Zero(t, h.client.SendCount(), "nothing should be submitted")
}

func TestReadWasmMissingFileNamesResolvedPath(t *testing.T) {
	h := newHarness(t, harnessOptions{})
	path := wasmPathOf(t, h)
	require.NoError(t, os.Remove(path))

	_, err := h.svc.Deploy(context.Background(), deploy.DeployRequest{Alias: "counter"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), path)
	assert.Contains(t, err.Error(), "has the contract been built?")
	assertNoRPC(t, h)
}

func TestReadWasmRejectsEmptyFile(t *testing.T) {
	h := newHarness(t, harnessOptions{})
	path := wasmPathOf(t, h)
	require.NoError(t, os.WriteFile(path, nil, 0o600))

	_, err := h.svc.Deploy(context.Background(), deploy.DeployRequest{Alias: "counter"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is empty")
	assert.Contains(t, err.Error(), path)
	assertNoRPC(t, h)
}

func TestReadWasmRejectsDirectory(t *testing.T) {
	h := newHarness(t, harnessOptions{})
	path := wasmPathOf(t, h)
	require.NoError(t, os.Remove(path))
	require.NoError(t, os.Mkdir(path, 0o755))

	_, err := h.svc.Deploy(context.Background(), deploy.DeployRequest{Alias: "counter"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read wasm "+path)
	assert.Contains(t, err.Error(), "is a directory")
	assertNoRPC(t, h)
}

// readWasm itself does not check the WASM magic bytes; checkWasm parses the
// module straight after it, so a non-WASM file still never reaches the RPC.
func TestReadWasmNonWasmFileRejectedBeforeRPC(t *testing.T) {
	h := newHarness(t, harnessOptions{})
	path := wasmPathOf(t, h)
	require.NoError(t, os.WriteFile(path, []byte("not a wasm module"), 0o600))

	_, err := h.svc.Deploy(context.Background(), deploy.DeployRequest{Alias: "counter"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "malformed wasm")
	assertNoRPC(t, h)
}
