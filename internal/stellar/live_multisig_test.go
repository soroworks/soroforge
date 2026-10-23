package stellar

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/txnbuild"
	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// liveMultisigEnv gates TestLiveMultisigUpload. Unlike the read-only smoke
// tests above, it submits transactions — with throwaway, friendbot-funded
// testnet accounts — so it has its own switch:
//
//	SOROFORGE_LIVE_MULTISIG=1 go test ./internal/stellar/ -run LiveMultisig -v
const liveMultisigEnv = "SOROFORGE_LIVE_MULTISIG"

// TestLiveMultisigUpload establishes, against testnet, that a 2-of-2 account
// can submit a Soroban transaction using the resource fee simulation returns,
// with no margin added for the second signature.
//
// Signatures are attached after simulation, and the resource fee includes a
// charge for transaction size. The question this answers is whether
// simulation already budgets for signatures it cannot see. The control case —
// the same transaction with only the primary's signature — must be refused
// for authorization, which proves the threshold is really in force.
func TestLiveMultisigUpload(t *testing.T) {
	if os.Getenv(liveMultisigEnv) == "" {
		t.Skipf("set %s=1 to run the live multisig test (submits to testnet)", liveMultisigEnv)
	}

	client, err := NewRPCClient(RPCOptions{URL: testnetRPC, NetworkPassphrase: testnetPassphrase})
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)

	primaryKey, err := keypair.Random()
	require.NoError(t, err)
	coKey, err := keypair.Random()
	require.NoError(t, err)
	friendbotFund(t, ctx, primaryKey.Address())

	// Make the account 2-of-2: master weight 1, co-signer weight 1, every
	// threshold 2. The account's thresholds are 0 before this transaction, so
	// the master key alone can authorize it.
	account, err := client.LoadAccount(ctx, primaryKey.Address())
	require.NoError(t, err)
	one, two := txnbuild.Threshold(1), txnbuild.Threshold(2)
	setup, err := txnbuild.NewTransaction(txnbuild.TransactionParams{
		SourceAccount:        account,
		IncrementSequenceNum: true,
		BaseFee:              txnbuild.MinBaseFee * 10,
		Preconditions:        txnbuild.Preconditions{TimeBounds: txnbuild.NewTimeout(120)},
		Operations: []txnbuild.Operation{&txnbuild.SetOptions{
			MasterWeight:    &one,
			LowThreshold:    &two,
			MediumThreshold: &two,
			HighThreshold:   &two,
			Signer:          &txnbuild.Signer{Address: coKey.Address(), Weight: one},
		}},
	})
	require.NoError(t, err)
	setup, err = setup.Sign(testnetPassphrase, primaryKey)
	require.NoError(t, err)
	submitAndAwait(t, ctx, client, setup)

	primary, err := NewKeypairSigner(primaryKey.Seed())
	require.NoError(t, err)
	cosigner, err := NewKeypairSigner(coKey.Seed())
	require.NoError(t, err)
	multi, err := NewMultiSigner(primary, cosigner)
	require.NoError(t, err)

	wasm, err := os.ReadFile(filepath.Join("..", "e2e", "testdata", "upgradeable", "v1.wasm"))
	require.NoError(t, err)

	assembled := assembleUpload(t, ctx, client, primaryKey.Address(), wasm)
	fee, err := ResourceFee(SimulateResult{TransactionDataXDR: sorobanDataXDR(t, assembled)})
	require.NoError(t, err)

	// Control: one signature on a 2-of-2 account is refused.
	single, err := primary.Sign(testnetPassphrase, assembled)
	require.NoError(t, err)
	singleXDR, err := single.Base64()
	require.NoError(t, err)
	sent, err := client.Send(ctx, singleXDR)
	require.NoError(t, err)
	assert.Equal(t, SendStatusError, sent.Status, "a single signature must not satisfy a 2-of-2 account")

	// Both signatures, simulated resource fee unchanged.
	signed, err := multi.Sign(testnetPassphrase, assembled)
	require.NoError(t, err)
	require.Len(t, signed.Signatures(), 2)
	result := submitAndAwait(t, ctx, client, signed)
	assert.Equal(t, TxStatusSuccess, result.Status)

	envelope, err := signed.MarshalBinary()
	require.NoError(t, err)
	t.Logf("2-of-2 upload succeeded in ledger %d: resource fee %d stroops (unmodified from simulation), envelope %d bytes",
		result.Ledger, fee, len(envelope))
}

func friendbotFund(t *testing.T, ctx context.Context, address string) {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://friendbot.stellar.org/?addr="+url.QueryEscape(address), nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "friendbot funding failed")
}

func assembleUpload(t *testing.T, ctx context.Context, client *RPCClient, source string, wasm []byte) *txnbuild.Transaction {
	t.Helper()
	account, err := client.LoadAccount(ctx, source)
	require.NoError(t, err)
	tx, err := Build(BuildParams{Source: account, Operation: UploadWasmOp(wasm, source)})
	require.NoError(t, err)
	txXDR, err := tx.Base64()
	require.NoError(t, err)
	sim, err := client.Simulate(ctx, txXDR)
	require.NoError(t, err)
	assembled, err := Assemble(tx, sim)
	require.NoError(t, err)
	return assembled
}

func sorobanDataXDR(t *testing.T, tx *txnbuild.Transaction) string {
	t.Helper()
	env := tx.ToXDR()
	require.NotNil(t, env.V1)
	data, ok := env.V1.Tx.Ext.GetSorobanData()
	require.True(t, ok, "assembled transaction carries soroban data")
	b64, err := xdr.MarshalBase64(data)
	require.NoError(t, err)
	return b64
}

func submitAndAwait(t *testing.T, ctx context.Context, client *RPCClient, tx *txnbuild.Transaction) TxResult {
	t.Helper()
	b64, err := tx.Base64()
	require.NoError(t, err)
	sent, err := client.Send(ctx, b64)
	require.NoError(t, err)
	require.NotEqual(t, SendStatusError, sent.Status, fmt.Sprintf("submission rejected: %s", sent.ErrorResultXDR))
	result, err := client.AwaitTransaction(ctx, sent.Hash)
	require.NoError(t, err)
	require.Equal(t, TxStatusSuccess, result.Status, "transaction %s did not succeed", sent.Hash)
	return result
}
