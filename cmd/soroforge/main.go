// Command soroforge deploys, upgrades, and tracks Soroban smart contracts.
//
// See the README for a quickstart, or run `soroforge --help`.
package main

import (
	"errors"
	"os"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		// Cobra has already printed the error, so this only decides the exit
		// code. Drift gets its own code so CI can distinguish "the check ran
		// and found a problem" from "the command failed".
		var drift *driftError
		if errors.As(err, &drift) {
			os.Exit(exitCodeDrift)
		}
		os.Exit(1)
	}
}

const (
	// exitCodeDrift is returned by `soroforge status` when a contract does not
	// match its recorded state.
	exitCodeDrift = 2
)

// driftError signals that a status check completed and found a mismatch. It is
// not a failure of the command, so it carries its own exit code.
type driftError struct {
	detail string
}

func (e *driftError) Error() string { return e.detail }
