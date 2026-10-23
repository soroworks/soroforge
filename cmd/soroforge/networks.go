package main

import (
	"errors"
	"time"

	"github.com/soroworks/soroforge/internal/config"
	"github.com/soroworks/soroforge/internal/deploy"
	"github.com/soroworks/soroforge/internal/netcheck"
	"github.com/spf13/cobra"
)

func newNetworksCmd() *cobra.Command {
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "networks",
		Short: "Check every configured network's RPC endpoint",
		Long: `Probe each network in soroforge.yaml and report whether it answered,
how fast, its latest ledger, and whether the passphrase the endpoint reports
matches the one configured.

A mistyped rpc_url or a provider that moved can point "testnet" at a different
chain; the failure then surfaces as a rejected signature or a deploy on the
wrong network. This command checks the mapping up front instead. Endpoints are
probed concurrently, each under --timeout.

The RPC URL is printed with any userinfo and query-string credentials removed,
so the report is safe to paste into a ticket or a CI log.

Exit codes:
  0  every network answered and matched its configuration
  1  at least one check failed (unreachable endpoint or passphrase mismatch)`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return err
			}

			// The same client the deploy service uses, so the probe exercises
			// precisely the endpoint a deploy would.
			results := netcheck.Check(cmd.Context(), cfg.Networks,
				netcheck.ClientFactory(deploy.DefaultClientFactory), timeout)

			if err := printNetworkResults(results); err != nil {
				return err
			}
			if netcheck.AnyFailure(results) {
				cmd.SilenceUsage = true
				return errors.New("one or more network checks failed")
			}
			return nil
		},
	}

	cmd.Flags().DurationVar(&timeout, "timeout", netcheck.DefaultTimeout,
		"timeout for each network probe")
	return cmd
}
