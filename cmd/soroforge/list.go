package main

import (
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List tracked contracts",
		Long: `Show every contract SoroForge tracks, across all networks.

Pass --network to restrict the list to one network.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			a, err := newApp(cmd.Context(), appOptions{})
			if err != nil {
				return err
			}
			defer a.Close()

			contracts, err := a.service.List(cmd.Context(), flags.network)
			if err != nil {
				return err
			}
			return printContracts(contracts)
		},
	}
}
