package deploy

// Close is the service lifecycle boundary: an edge case here leaks RPC clients
// until the process exits, or reports a successful shutdown when one failed.
// These tests sit in the package so they can install cached clients directly,
// including ones that do not implement io.Closer.

import (
	"errors"
	"testing"

	"github.com/soroworks/soroforge/internal/config"
	"github.com/soroworks/soroforge/internal/stellar"
	"github.com/soroworks/soroforge/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// clientWithoutClose satisfies stellar.Client but not io.Closer, modelling a
// client implementation that needs no explicit cleanup.
type clientWithoutClose struct{ stellar.Client }

func TestServiceCloseEdgeCases(t *testing.T) {
	testnetErr := errors.New("testnet RPC unavailable")
	mainnetErr := errors.New("mainnet RPC unavailable")

	tests := map[string]struct {
		clients map[string]stellar.Client
		wantErr []string
	}{
		"no cached clients": {
			clients: nil,
		},
		"close errors name the network and are all reported": {
			clients: map[string]stellar.Client{
				"testnet": &stellar.MockClient{CloseErr: testnetErr},
				"mainnet": &stellar.MockClient{CloseErr: mainnetErr},
			},
			wantErr: []string{
				"close testnet RPC client: " + testnetErr.Error(),
				"close mainnet RPC client: " + mainnetErr.Error(),
			},
		},
		"clients without Close are skipped": {
			clients: map[string]stellar.Client{
				"testnet": clientWithoutClose{},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			svc, err := New(Options{
				Config: &config.Config{},
				Store:  store.NewMemoryStore(),
			})
			require.NoError(t, err)
			svc.clientCache = tc.clients

			err = svc.Close()
			if len(tc.wantErr) == 0 {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				for _, want := range tc.wantErr {
					assert.ErrorContains(t, err, want)
				}
			}

			assert.Nil(t, svc.clientCache, "a closed service must drop its client references")
			assert.NoError(t, svc.Close(), "closing an already-closed service is a no-op")
		})
	}
}
