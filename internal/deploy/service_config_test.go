package deploy

import (
	"testing"

	"github.com/soroworks/soroforge/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Config is a read-only accessor, but its edge cases still matter: callers use
// it to render network and contract names, so it must never invent a config or
// hand back a copy that could drift from the one the service uses.
func TestServiceConfigEdgeCases(t *testing.T) {
	empty := &config.Config{}
	populated := &config.Config{
		Version:        config.SupportedVersion,
		DefaultNetwork: "testnet",
		Networks: map[string]config.Network{
			"testnet": {RPCURL: "https://rpc.example.org", Passphrase: "Test SDF Network ; September 2015"},
		},
		Contracts: map[string]config.Contract{
			"counter": {Wasm: "./counter.wasm"},
		},
	}

	tests := map[string]struct {
		svc  *Service
		want *config.Config
	}{
		"zero-value service has no config": {
			svc:  &Service{},
			want: nil,
		},
		"empty config is returned as-is": {
			svc:  &Service{cfg: empty},
			want: empty,
		},
		"populated config keeps its networks and contracts": {
			svc:  &Service{cfg: populated},
			want: populated,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := tc.svc.Config()
			if tc.want == nil {
				assert.Nil(t, got)
				return
			}

			require.NotNil(t, got)
			assert.Same(t, tc.want, got, "Config must expose the service's own config, not a copy")
			assert.Equal(t, tc.want.Networks, got.Networks)
			assert.Equal(t, tc.want.Contracts, got.Contracts)
		})
	}
}
