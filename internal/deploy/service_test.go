package deploy

import (
	"fmt"
	"time"

	"github.com/soroworks/soroforge/internal/config"
	"github.com/soroworks/soroforge/internal/store"
	"github.com/stellar/go-stellar-sdk/txnbuild"
)

// ExampleNew shows how to create a minimal deploy Service for read-only
// operations like List and History.
func ExampleNew() {
	cfg := &config.Config{
		DefaultNetwork: "testnet",
		Networks: map[string]config.Network{
			"testnet": {
				RPCURL:       "https://soroban-testnet.stellar.org",
				Passphrase:   "Test SDF Network ; September 2015",
				FriendbotURL: "https://friendbot.stellar.org",
			},
		},
		Contracts: map[string]config.Contract{
			"counter": {Wasm: "./target/wasm32v1-none/release/counter.wasm"},
		},
	}

	store := store.NewMemoryStore()

	svc, err := New(Options{
		Config: cfg,
		Store:  store,
	})
	if err != nil {
		panic(err)
	}

	fmt.Printf("default network: %s\n", svc.Config().DefaultNetwork)
	fmt.Printf("contracts: %d\n", len(svc.Config().Contracts))
	_ = txnbuild.MinBaseFee // referenced for side-effect-free compilation

	// Output:
	// default network: testnet
	// contracts: 1
}

// ExampleNew_withSigner shows how to create a Service with a signer for
// deploy and upgrade operations.
func ExampleNew_withSigner() {
	cfg := &config.Config{
		DefaultNetwork: "testnet",
		Networks: map[string]config.Network{
			"testnet": {RPCURL: "https://soroban-testnet.stellar.org"},
		},
	}

	store := store.NewMemoryStore()

	svc, err := New(Options{
		Config:         cfg,
		Store:          store,
		BaseFee:        txnbuild.MinBaseFee,
		ConfirmTimeout: 30 * time.Second,
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(svc.Config().DefaultNetwork)

	// Output: testnet
}
