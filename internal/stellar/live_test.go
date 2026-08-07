package stellar

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/txnbuild"
	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A smoke test against live Stellar testnet, skipped unless SOROFORGE_LIVE is
// set. `go test ./...` must never need a network, so this is opt-in:
//
//	SOROFORGE_LIVE=1 go test ./internal/stellar/ -run Live -v
//
// It exists because the rest of the suite proves SoroForge is internally
// consistent, not that its understanding of the RPC API is still correct. This
// catches the failure the mocks cannot: an SDK or RPC change that alters real
// response shapes. Run it after upgrading github.com/stellar/go-stellar-sdk.
//
// Read-only: no keys, no submissions, no fees.
const liveEnv = "SOROFORGE_LIVE"

// Public SDF-hosted testnet endpoint. There is no equivalent public mainnet RPC.
const (
	testnetRPC        = "https://soroban-testnet.stellar.org"
	testnetPassphrase = "Test SDF Network ; September 2015"
)

func liveClient(t *testing.T) (*RPCClient, context.Context) {
	t.Helper()
	if os.Getenv(liveEnv) == "" {
		t.Skipf("set %s=1 to run live testnet smoke tests", liveEnv)
	}

	client, err := NewRPCClient(RPCOptions{
		URL:               testnetRPC,
		NetworkPassphrase: testnetPassphrase,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	t.Cleanup(cancel)

	return client, ctx
}

func TestLiveLatestLedger(t *testing.T) {
	client, ctx := liveClient(t)

	sequence, err := client.LatestLedger(ctx)
	require.NoError(t, err)
	assert.NotZero(t, sequence, "testnet should report a closed ledger")
	t.Logf("testnet latest ledger: %d", sequence)
}

func TestLiveContractInstanceDecoding(t *testing.T) {
	// The native XLM Stellar Asset Contract is guaranteed to exist on testnet,
	// which makes it a stable target for exercising the drift-detection read
	// path against real ledger data: key construction, entry lookup, and
	// decoding down to the instance executable.
	client, ctx := liveClient(t)

	asset, err := txnbuild.NativeAsset{}.ToXDR()
	require.NoError(t, err)
	rawID, err := asset.ContractID(testnetPassphrase)
	require.NoError(t, err)
	contractID, err := strkey.Encode(strkey.VersionByteContract, rawID[:])
	require.NoError(t, err)
	t.Logf("native SAC on testnet: %s", contractID)

	key, err := InstanceLedgerKey(contractID)
	require.NoError(t, err)
	encodedKey, err := xdr.MarshalBase64(key)
	require.NoError(t, err)

	entries, err := client.LedgerEntries(ctx, []string{encodedKey})
	require.NoError(t, err)
	require.Len(t, entries, 1, "InstanceLedgerKey must locate the native SAC's instance entry")

	// A Stellar Asset Contract has no uploaded bytecode, so reaching this
	// specific error proves the decode chain got all the way to the executable.
	_, err = OnChainWasmHash(ctx, client, contractID)
	assert.ErrorContains(t, err, "Stellar Asset Contract")
}

func TestLiveAbsentContractIsNotFound(t *testing.T) {
	client, ctx := liveClient(t)

	absent, err := strkey.Encode(strkey.VersionByteContract, make([]byte, 32))
	require.NoError(t, err)

	_, err = OnChainWasmHash(ctx, client, absent)

	var notFound *ErrContractNotFound
	assert.ErrorAs(t, err, &notFound,
		"a contract that does not exist must be reported as not found, not as a failure")
}
