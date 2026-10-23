package main

import (
	"fmt"

	"github.com/soroworks/soroforge/internal/deploy"
	"github.com/spf13/cobra"
)

func newDiffCmd() *cobra.Command {
	var (
		from           string
		to             string
		failOnBreaking bool
	)

	cmd := &cobra.Command{
		Use:   "diff <alias>",
		Short: "Show the interface change between recorded versions",
		Long: `Show what an upgrade changed in a contract's interface: functions, types and
events added, removed, or changed between two recorded WASM versions.

The interfaces are read from the network's sorovault_url, where SoroVault keeps
the decoded interface of every version it has registered. By default the last
two recorded versions are compared; --from and --to pick any pair.

Removed or changed functions and types are marked breaking. With
--fail-on-breaking the command exits non-zero when there are any, so it can
gate an upgrade in CI.

Exit codes:
  0  compared (with --fail-on-breaking: no breaking changes)
  1  the comparison could not be made
  2  breaking changes found (only with --fail-on-breaking)`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := newApp(cmd.Context(), appOptions{})
			if err != nil {
				return err
			}
			defer a.Close()

			result, err := a.service.Diff(cmd.Context(), deploy.DiffRequest{
				Alias:   args[0],
				Network: flags.network,
				From:    from,
				To:      to,
			})
			if err != nil {
				return err
			}
			if err := printDiff(result); err != nil {
				return err
			}

			if failOnBreaking && result.Breaking {
				// Printed already; this only selects the exit code.
				cmd.SilenceErrors = true
				return &driftError{detail: fmt.Sprintf("%s has breaking interface changes", result.Alias)}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&from, "from", "", "WASM hash of the older version (default: the version before --to)")
	cmd.Flags().StringVar(&to, "to", "", "WASM hash of the newer version (default: the current version)")
	cmd.Flags().BoolVar(&failOnBreaking, "fail-on-breaking", false, "exit 2 when any change is breaking")
	attachAliasCompletion(cmd)
	return cmd
}
