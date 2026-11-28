package stellar_test

import (
	"context"
	"fmt"
	"time"

	"github.com/soroworks/soroforge/internal/stellar"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// ExampleNewMockSigner creates a signer backed by a generated keypair.
func ExampleNewMockSigner() {
	signer := stellar.NewMockSigner()
	fmt.Println(len(signer.Address()) == 56)

	// Output:
	// true
}

// ExampleNewMockSignerFromSeed creates a signer with a stable address from a
// Stellar secret seed, which is useful when a test needs repeatable identities.
func ExampleNewMockSignerFromSeed() {
	signer, err := stellar.NewMockSignerFromSeed("SBFGFF27Y64ZUGFAIG5AMJGQODZZKV2YQKAVUUN4HNE24XZXD2OEUVUP")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(len(signer.Address()) == 56)

	// Output:
	// true
}

// ExampleNewMultiSigner combines a primary signer with one or more
// co-signers for transactions that require multiple signatures.
func ExampleNewMultiSigner() {
	primary, err := stellar.NewMockSignerFromSeed("SBFGFF27Y64ZUGFAIG5AMJGQODZZKV2YQKAVUUN4HNE24XZXD2OEUVUP")
	if err != nil {
		fmt.Println(err)
		return
	}
	coSigner, err := stellar.NewMockSignerFromSeed("SCN4VT2ELMFTALLNPPJNCK6OGQEWLERZIMEPARTRVDXXL2IJAPWSXZQI")
	if err != nil {
		fmt.Println(err)
		return
	}
	multi, err := stellar.NewMultiSigner(primary, coSigner)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(multi.Signers())

	// Output:
	// 2
}

// ExampleContractIDFromSimulation recovers the contract ID a create-contract
// simulation reports, so a dry run can print the address a real deploy would
// produce before anything is submitted.
func ExampleContractIDFromSimulation() {
	// The simulated return value is the new contract's address, encoded as
	// base64 XDR. ContractIDFromSimulation decodes it back into a C... strkey.
	encoded, err := xdr.MarshalBase64(xdr.ScVal{
		Type: xdr.ScValTypeScvAddress,
		Address: &xdr.ScAddress{
			Type: xdr.ScAddressTypeScAddressTypeContract,
			ContractId: func() *xdr.ContractId {
				raw := make([]byte, 32)
				for i := range raw {
					raw[i] = byte(i + 1)
				}
				id := xdr.ContractId(raw)
				return &id
			}(),
		},
	})
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	got, err := stellar.ContractIDFromSimulation(stellar.SimulateResult{
		Results: []stellar.HostFunctionResult{{ReturnValueXDR: encoded}},
	})
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(got)

	// Output:
	// CAAQEAYEAUDAOCAJBIFQYDIOB4IBCEQTCQKRMFYYDENBWHA5DYPSBFLM
}

// ExampleContractIDFromScVal renders an ScVal that already holds a contract
// address — the form a return value or a contract-data key arrives in — as a
// C... strkey.
func ExampleContractIDFromScVal() {
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(i + 1)
	}
	id := xdr.ContractId(raw)

	got, err := stellar.ContractIDFromScVal(xdr.ScVal{
		Type: xdr.ScValTypeScvAddress,
		Address: &xdr.ScAddress{
			Type:       xdr.ScAddressTypeScAddressTypeContract,
			ContractId: &id,
		},
	})
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(got)

	// Output:
	// CAAQEAYEAUDAOCAJBIFQYDIOB4IBCEQTCQKRMFYYDENBWHA5DYPSBFLM
}

// ExampleNewMockClient builds a Client that answers from canned data, so code
// that builds and submits transactions can be exercised without an RPC
// endpoint.
func ExampleNewMockClient() {
	ctx := context.Background()
	client := stellar.NewMockClient()

	// The defaults model a healthy testnet endpoint: the testnet passphrase, a
	// sequence number to build transactions against, and a confirmation that
	// succeeds.
	fmt.Println(client.NetworkPassphrase())

	account, err := client.LoadAccount(ctx, "GAYUB4KATGTUZEGUMJEOZDPPWM4MQLHCIKC4T55YSXHN234WI6BJMIY2")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	seq, err := account.GetSequenceNumber()
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("sequence:", seq)

	sent, err := client.Send(ctx, "signed-envelope-xdr")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("send:", sent.Status, "in ledger", sent.LatestLedger)

	result, err := client.AwaitTransaction(ctx, sent.Hash)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("await:", result.Status)

	// Calls are recorded, so a test can assert on what the code under test did.
	fmt.Println("sent:", client.SendCount())

	// Output:
	// Test SDF Network ; September 2015
	// sequence: 1
	// send: PENDING in ledger 1000
	// await: SUCCESS
	// sent: 1
}

// ExampleNewKeypairSigner builds a signer from an existing Stellar secret seed.
//
// SoroForge never generates or stores a key: it reads one someone else
// provisioned — from an environment variable or a keystore file — and keeps it
// unexported.
func ExampleNewKeypairSigner() {
	// A throwaway test key. Never commit a seed that protects real funds.
	const seed = "SBFGFF27Y64ZUGFAIG5AMJGQODZZKV2YQKAVUUN4HNE24XZXD2OEUVUP"

	signer, err := stellar.NewKeypairSigner(seed)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	// Only the public half is ever exposed; the seed is never returned,
	// logged, or serialised.
	fmt.Println(signer.Address())
	fmt.Println(signer)

	// Output:
	// GAYUB4KATGTUZEGUMJEOZDPPWM4MQLHCIKC4T55YSXHN234WI6BJMIY2
	// KeypairSigner(GAYUB4KATGTUZEGUMJEOZDPPWM4MQLHCIKC4T55YSXHN234WI6BJMIY2)
}

// ExampleNewCommandSigner builds a signer that delegates signing to an external
// executable, for a key that cannot be read into this process — an HSM, a cloud
// KMS, or a remote signing service.
func ExampleNewCommandSigner() {
	signer, err := stellar.NewCommandSigner(stellar.CommandSignerOptions{
		// An executable followed by optional arguments, run directly rather
		// than through a shell.
		Command: "hsm-signer sign --key deployer",
		// The public key cannot be derived without the private key, so it is
		// configured here; every signature is verified against it before it is
		// attached to the transaction.
		Address: "GAYUB4KATGTUZEGUMJEOZDPPWM4MQLHCIKC4T55YSXHN234WI6BJMIY2",
		// Bounds one signing invocation, so a hung device cannot stall a deploy.
		Timeout: 10 * time.Second,
	})
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	// The signer exposes only the configured address: the private key stays
	// with the command.
	fmt.Println(signer.Address())

	// Output:
	// GAYUB4KATGTUZEGUMJEOZDPPWM4MQLHCIKC4T55YSXHN234WI6BJMIY2
}
