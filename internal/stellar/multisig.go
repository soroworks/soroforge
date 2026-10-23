package stellar

import (
	"errors"
	"fmt"

	"github.com/stellar/go-stellar-sdk/txnbuild"
)

// maxEnvelopeSignatures is the protocol's cap on signatures per transaction
// envelope.
const maxEnvelopeSignatures = 20

// MultiSigner signs for a deployer account protected by multisig thresholds:
// a primary signer, whose address is the transaction source, plus co-signers
// whose signatures together meet the account's threshold. Every signer's
// signature is applied.
//
// Fees need no adjustment for the extra signatures, even though they are
// attached after simulation: TestLiveMultisigUpload submits a 2-of-2 Soroban
// transaction on testnet with the simulated resource fee unchanged, and it
// succeeds. Assemble therefore adds no size-based margin.
type MultiSigner struct {
	primary   Signer
	cosigners []Signer
}

var _ Signer = (*MultiSigner)(nil)

// NewMultiSigner combines a primary signer with co-signers.
//
// A key may appear only once: a duplicate signature is not just wasted, it
// makes the network reject the transaction (txBAD_AUTH_EXTRA), so it is caught
// here, before anything is built.
func NewMultiSigner(primary Signer, cosigners ...Signer) (*MultiSigner, error) {
	if primary == nil {
		return nil, errors.New("multisig: no primary signer")
	}
	if len(cosigners)+1 > maxEnvelopeSignatures {
		return nil, fmt.Errorf("multisig: %d signers exceeds the %d signatures a transaction can carry",
			len(cosigners)+1, maxEnvelopeSignatures)
	}

	seen := map[string]bool{primary.Address(): true}
	for i, c := range cosigners {
		if c == nil {
			return nil, fmt.Errorf("multisig: co-signer %d is nil", i+1)
		}
		if seen[c.Address()] {
			return nil, fmt.Errorf("multisig: %s is configured more than once; each key may sign only once", c.Address())
		}
		seen[c.Address()] = true
	}
	return &MultiSigner{primary: primary, cosigners: append([]Signer(nil), cosigners...)}, nil
}

// Address returns the primary signer's address: the account that is the
// transaction source and is recorded as the deployer.
func (m *MultiSigner) Address() string { return m.primary.Address() }

// Signers reports how many signatures Sign applies.
func (m *MultiSigner) Signers() int { return len(m.cosigners) + 1 }

// Sign returns a copy of tx signed by the primary and then every co-signer.
func (m *MultiSigner) Sign(networkPassphrase string, tx *txnbuild.Transaction) (*txnbuild.Transaction, error) {
	signed, err := m.primary.Sign(networkPassphrase, tx)
	if err != nil {
		return nil, err
	}
	for _, c := range m.cosigners {
		signed, err = c.Sign(networkPassphrase, signed)
		if err != nil {
			return nil, fmt.Errorf("co-signer %s: %w", c.Address(), err)
		}
	}
	return signed, nil
}

// String names the signers by address only, like KeypairSigner.
func (m *MultiSigner) String() string {
	return fmt.Sprintf("MultiSigner(%s + %d co-signers)", m.primary.Address(), len(m.cosigners))
}
