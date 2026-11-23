package deploy

import (
	"context"

	"github.com/soroworks/soroforge/internal/netcheck"
)

// CheckNetworks probes every configured network's RPC endpoint concurrently
// using the service's client factory. It returns one credential-redacted
// report per network, sorted by name, and never fails: an unreachable endpoint
// is a result with OK false, not an error that suppresses the other reports.
func (s *Service) CheckNetworks(ctx context.Context) []netcheck.Result {
	return netcheck.Check(ctx, s.cfg.Networks, netcheck.ClientFactory(s.clients), 0)
}
