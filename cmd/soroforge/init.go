package main

import (
	"fmt"
	"path/filepath"

	"github.com/soroworks/soroforge/internal/config"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	var dir string
	var force bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Write a starter soroforge.yaml for the contracts in this project",
		Long: `Write a starter soroforge.yaml.

init looks for release WASM under target/wasm32v1-none/release (current Soroban
toolchains) and target/wasm32-unknown-unknown/release (older ones), preferring
a .optimized.wasm when there is one, and declares one contract per artifact,
named after its file. With nothing built yet, it writes a placeholder to edit.

The file targets testnet, needs no secrets, and is meant to be committed. An
existing file is never replaced without --force.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path := flags.configPath
			if path == "" {
				path = filepath.Join(dir, config.DefaultFilename)
			}

			found, err := config.WriteScaffold(dir, path, force)
			if err != nil {
				return err
			}

			w := out()
			fmt.Fprintf(w, "Wrote %s.\n\n", path)
			if len(found) == 0 {
				fmt.Fprintln(w, "No built contracts found under target/; edit the placeholder after `stellar contract build`.")
			} else {
				for _, f := range found {
					fmt.Fprintf(w, "  %-20s %s\n", f.Alias, f.Wasm)
				}
			}
			fmt.Fprintln(w, "\nNext: soroforge deploy <alias> --dry-run")
			return nil
		},
	}

	cmd.Flags().StringVar(&dir, "dir", ".", "project directory to scan for built contracts")
	cmd.Flags().BoolVar(&force, "force", false, "replace an existing soroforge.yaml")
	return cmd
}
