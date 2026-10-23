package stellar

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/txnbuild"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func randomSigner(t *testing.T) (*KeypairSigner, *keypair.Full) {
	t.Helper()
	full, err := keypair.Random()
	require.NoError(t, err)
	s, err := NewKeypairSigner(full.Seed())
	require.NoError(t, err)
	return s, full
}

func multisigTestTx(t *testing.T, source string) *txnbuild.Transaction {
	t.Helper()
	tx, err := txnbuild.NewTransaction(txnbuild.TransactionParams{
		SourceAccount:        &txnbuild.SimpleAccount{AccountID: source, Sequence: 1},
		IncrementSequenceNum: true,
		Operations:           []txnbuild.Operation{&txnbuild.BumpSequence{BumpTo: 2}},
		BaseFee:              txnbuild.MinBaseFee,
		Preconditions:        txnbuild.Preconditions{TimeBounds: txnbuild.NewInfiniteTimeout()},
	})
	require.NoError(t, err)
	return tx
}

func TestMultiSignerAppliesEverySignature(t *testing.T) {
	primary, primaryKey := randomSigner(t)
	co1, co1Key := randomSigner(t)
	co2, co2Key := randomSigner(t)

	m, err := NewMultiSigner(primary, co1, co2)
	require.NoError(t, err)
	assert.Equal(t, primary.Address(), m.Address(), "the source account is the primary's")
	assert.Equal(t, 3, m.Signers())

	tx := multisigTestTx(t, primary.Address())
	signed, err := m.Sign(network.TestNetworkPassphrase, tx)
	require.NoError(t, err)
	require.Len(t, signed.Signatures(), 3)
	assert.Empty(t, tx.Signatures(), "the input transaction is not modified")

	hash, err := signed.Hash(network.TestNetworkPassphrase)
	require.NoError(t, err)
	for i, key := range []*keypair.Full{primaryKey, co1Key, co2Key} {
		sig := signed.Signatures()[i]
		assert.Equal(t, key.Hint(), [4]byte(sig.Hint), "signature %d is from the expected key", i)
		assert.NoError(t, key.Verify(hash[:], sig.Signature), "signature %d verifies", i)
	}
}

func TestMultiSignerWithNoCosignersSignsOnce(t *testing.T) {
	primary, _ := randomSigner(t)
	m, err := NewMultiSigner(primary)
	require.NoError(t, err)

	signed, err := m.Sign(network.TestNetworkPassphrase, multisigTestTx(t, primary.Address()))
	require.NoError(t, err)
	assert.Len(t, signed.Signatures(), 1)
}

func TestNewMultiSignerRejectsDuplicateKeys(t *testing.T) {
	primary, _ := randomSigner(t)
	co, _ := randomSigner(t)

	_, err := NewMultiSigner(primary, co, co)
	require.Error(t, err)
	assert.Contains(t, err.Error(), co.Address())
	assert.Contains(t, err.Error(), "more than once")

	// The primary repeated as a co-signer is a duplicate too.
	_, err = NewMultiSigner(primary, primary)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "more than once")
}

func TestNewMultiSignerRejectsBadInput(t *testing.T) {
	_, err := NewMultiSigner(nil)
	assert.ErrorContains(t, err, "no primary signer")

	primary, _ := randomSigner(t)
	_, err = NewMultiSigner(primary, nil)
	assert.ErrorContains(t, err, "co-signer 1 is nil")

	cosigners := make([]Signer, maxEnvelopeSignatures)
	for i := range cosigners {
		cosigners[i], _ = randomSigner(t)
	}
	_, err = NewMultiSigner(primary, cosigners...)
	assert.ErrorContains(t, err, "exceeds")
}

type failingSigner struct{ address string }

func (f failingSigner) Address() string { return f.address }
func (f failingSigner) Sign(string, *txnbuild.Transaction) (*txnbuild.Transaction, error) {
	return nil, errors.New("hsm unavailable")
}

func TestMultiSignerReportsWhichCosignerFailed(t *testing.T) {
	primary, _ := randomSigner(t)
	_, other := randomSigner(t)
	bad := failingSigner{address: other.Address()}

	m, err := NewMultiSigner(primary, bad)
	require.NoError(t, err)

	_, err = m.Sign(network.TestNetworkPassphrase, multisigTestTx(t, primary.Address()))
	require.Error(t, err)
	assert.Contains(t, err.Error(), bad.Address())
	assert.Contains(t, err.Error(), "hsm unavailable")

	// A primary failure is returned as-is.
	_, err = m.Sign("", multisigTestTx(t, primary.Address()))
	assert.ErrorContains(t, err, "empty network passphrase")
}

func TestMultiSignerStringShowsNoSeeds(t *testing.T) {
	primary, primaryKey := randomSigner(t)
	co, coKey := randomSigner(t)
	m, err := NewMultiSigner(primary, co)
	require.NoError(t, err)

	for _, s := range []string{m.String(), fmt.Sprintf("%v", m), fmt.Sprintf("%+v", m)} {
		assert.NotContains(t, s, primaryKey.Seed())
		assert.NotContains(t, s, coKey.Seed())
	}
	assert.Contains(t, m.String(), primary.Address())
}
