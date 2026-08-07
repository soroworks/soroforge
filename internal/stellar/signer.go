package stellar

import (
	"fmt"

	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/txnbuild"
)

// KeypairSigner signs with a Stellar secret seed held in memory.
//
// This is the only Signer SoroForge ships, and it deliberately does no key
// management: it reads a key someone else provisioned (an environment variable
// or a keystore file — see config.Env.SigningSeed) and signs with it. Key
// generation, rotation, and storage are left to the tools built for that job.
//
// The seed is kept unexported and is never returned, logged, or serialised. The
// only thing this type exposes about the key is its public address.
type KeypairSigner struct {
	full *keypair.Full
}

var _ Signer = (*KeypairSigner)(nil)

// NewKeypairSigner parses a Stellar secret seed (S...).
//
// The error deliberately does not echo the input: a malformed seed is still
// secret, and an error string tends to end up in logs and CI output.
func NewKeypairSigner(seed string) (*KeypairSigner, error) {
	full, err := keypair.ParseFull(seed)
	if err != nil {
		return nil, fmt.Errorf("parse signing key: invalid Stellar secret seed (expected an S... value)")
	}
	return &KeypairSigner{full: full}, nil
}

// Address returns the signer's public G... address. This is the only part of
// the key that is safe to record, and it is what gets written to the deployment
// history as deployer_pubkey.
func (s *KeypairSigner) Address() string { return s.full.Address() }

// Sign returns a signed copy of tx.
func (s *KeypairSigner) Sign(networkPassphrase string, tx *txnbuild.Transaction) (*txnbuild.Transaction, error) {
	if tx == nil {
		return nil, fmt.Errorf("sign: nil transaction")
	}
	if networkPassphrase == "" {
		return nil, fmt.Errorf("sign: empty network passphrase")
	}
	signed, err := tx.Sign(networkPassphrase, s.full)
	if err != nil {
		return nil, fmt.Errorf("sign transaction: %w", err)
	}
	return signed, nil
}

// String keeps the seed out of accidental formatting: fmt verbs on a
// KeypairSigner print this instead of the struct's contents.
func (s *KeypairSigner) String() string {
	return fmt.Sprintf("KeypairSigner(%s)", s.full.Address())
}
