package deploy_test

import (
	"context"
	"math"
	"testing"

	"github.com/soroworks/soroforge/internal/deploy"
	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Extend resolves an alias and caps a ledger count, so its edge cases are the
// requests that cannot resolve and the values at either end of the uint32
// range the protocol accepts.
func TestExtendEdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		req          deploy.ExtendRequest
		maxTTL       uint32
		wantExtended uint32
		wantCapped   bool
		wantErr      string
	}{
		{
			name:    "empty request has no alias to resolve",
			req:     deploy.ExtendRequest{},
			maxTTL:  1_000_000,
			wantErr: "unknown contract",
		},
		{
			name:         "request exactly at the network cap is not capped",
			req:          deploy.ExtendRequest{Alias: "counter", Ledgers: 99_999},
			maxTTL:       100_000,
			wantExtended: 99_999,
			wantCapped:   false,
		},
		{
			name:         "maximum uint32 request is capped without overflow",
			req:          deploy.ExtendRequest{Alias: "counter", Ledgers: math.MaxUint32},
			maxTTL:       100_000,
			wantExtended: 99_999,
			wantCapped:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, harnessOptions{})
			trackLiveContract(t, h, h.wasmHash)
			require.NoError(t, h.client.SetMaxEntryTTL(tc.maxTTL))

			result, err := h.svc.Extend(context.Background(), tc.req)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, tc.wantErr)
				assert.Nil(t, result)
				assert.Zero(t, h.client.SendCount(), "an unresolvable request must not submit anything")
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantExtended, result.ExtendedLedgers)
			assert.Equal(t, tc.wantCapped, result.Capped)

			envelope := lastSentEnvelope(t, h)
			extend, ok := envelope.V1.Tx.Operations[0].Body.GetExtendFootprintTtlOp()
			require.True(t, ok)
			assert.Equal(t, xdr.Uint32(tc.wantExtended), extend.ExtendTo,
				"the submitted value must be the capped one")
		})
	}
}
