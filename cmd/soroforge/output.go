package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/soroworks/soroforge/internal/deploy"
	"github.com/soroworks/soroforge/internal/store"
)

// Output goes to stdout; logs go to stderr. That split is what lets
// `soroforge list --json | jq` work while logging is still on.
//
// Nothing printed here includes key material. Results carry the deployer's
// public address and never the seed that produced it.

func out() io.Writer { return os.Stdout }

// printJSON writes an indented JSON document.
func printJSON(v any) error {
	enc := json.NewEncoder(out())
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// shortHash abbreviates a 64-character hash for table output. The full value is
// always available with --json, so the table optimises for scanning.
func shortHash(hash string) string {
	const shown = 12
	if len(hash) <= shown {
		return hash
	}
	return hash[:shown] + "…"
}

func printDeployResult(r *deploy.DeployResult) error {
	if flags.jsonOutput {
		return printJSON(r)
	}

	w := out()
	if r.DryRun {
		fmt.Fprintf(w, "Dry run — nothing was submitted and nothing was recorded.\n\n")
	} else {
		fmt.Fprintf(w, "Deployed %s to %s.\n\n", r.Alias, r.Network)
	}

	fmt.Fprintf(w, "  Contract ID   %s\n", r.ContractID)
	fmt.Fprintf(w, "  WASM hash     %s\n", r.WasmHash)
	fmt.Fprintf(w, "  Network       %s\n", r.Network)
	fmt.Fprintf(w, "  Deployer      %s\n", r.Deployer)

	if r.UploadSkipped {
		fmt.Fprintf(w, "  Upload        skipped (this bytecode is already on-chain)\n")
	} else if r.UploadTxHash != "" {
		fmt.Fprintf(w, "  Upload tx     %s\n", r.UploadTxHash)
	}
	if r.CreateTxHash != "" {
		fmt.Fprintf(w, "  Create tx     %s\n", r.CreateTxHash)
	}
	if r.Ledger != 0 {
		fmt.Fprintf(w, "  Ledger        %d\n", r.Ledger)
	}
	printExtension(w, r.Alias, r.Extension, r.ExtensionError)
	printCatalog(w, r.Catalog)

	if r.DryRun {
		fmt.Fprintf(w, "\nAssembled transaction envelopes (unsigned):\n")
		if r.UploadEnvelopeXDR != "" {
			fmt.Fprintf(w, "\n  upload:\n  %s\n", r.UploadEnvelopeXDR)
		}
		if r.CreateEnvelopeXDR != "" {
			fmt.Fprintf(w, "\n  create:\n  %s\n", r.CreateEnvelopeXDR)
		}
		fmt.Fprintf(w, "\nRe-run without --dry-run to submit.\n")
	}

	return nil
}

func printUpgradeResult(r *deploy.UpgradeResult) error {
	if flags.jsonOutput {
		return printJSON(r)
	}

	w := out()

	if r.NoChange {
		fmt.Fprintf(w, "%s on %s already runs this WASM hash — nothing to do.\n\n",
			r.Alias, r.Network)
		fmt.Fprintf(w, "  Contract ID   %s\n", r.ContractID)
		fmt.Fprintf(w, "  WASM hash     %s\n", r.WasmHash)
		return nil
	}

	if r.DryRun {
		fmt.Fprintf(w, "Dry run — nothing was submitted and nothing was recorded.\n\n")
	} else {
		fmt.Fprintf(w, "Upgraded %s on %s.\n\n", r.Alias, r.Network)
	}

	fmt.Fprintf(w, "  Contract ID   %s\n", r.ContractID)
	fmt.Fprintf(w, "  From          %s\n", r.PreviousWasmHash)
	fmt.Fprintf(w, "  To            %s\n", r.WasmHash)
	fmt.Fprintf(w, "  Entrypoint    %s\n", r.UpgradeFunc)
	fmt.Fprintf(w, "  Deployer      %s\n", r.Deployer)

	if r.UploadSkipped {
		fmt.Fprintf(w, "  Upload        skipped (this bytecode is already on-chain)\n")
	} else if r.UploadTxHash != "" {
		fmt.Fprintf(w, "  Upload tx     %s\n", r.UploadTxHash)
	}
	if r.UpgradeTxHash != "" {
		fmt.Fprintf(w, "  Upgrade tx    %s\n", r.UpgradeTxHash)
	}
	if r.Ledger != 0 {
		fmt.Fprintf(w, "  Ledger        %d\n", r.Ledger)
	}
	printExtension(w, r.Alias, r.Extension, r.ExtensionError)
	printCatalog(w, r.Catalog)

	if r.DryRun && r.UpgradeEnvelopeXDR != "" {
		fmt.Fprintf(w, "\nAssembled upgrade envelope (unsigned):\n\n  %s\n", r.UpgradeEnvelopeXDR)
		fmt.Fprintf(w, "\nRe-run without --dry-run to submit.\n")
	}

	return nil
}

func printUploadResult(r *deploy.UploadResult) error {
	if flags.jsonOutput {
		return printJSON(r)
	}

	w := out()

	if r.UploadSkipped {
		fmt.Fprintf(w, "%s's bytecode is already on-chain on %s — nothing to upload.\n\n",
			r.Alias, r.Network)
		fmt.Fprintf(w, "  WASM hash     %s\n", r.WasmHash)
		fmt.Fprintf(w, "  Deployer      %s\n", r.Deployer)
		return nil
	}

	if r.DryRun {
		fmt.Fprintf(w, "Dry run — nothing was submitted and nothing was recorded.\n\n")
	} else {
		fmt.Fprintf(w, "Uploaded %s's bytecode to %s.\n\n", r.Alias, r.Network)
	}

	fmt.Fprintf(w, "  WASM hash     %s\n", r.WasmHash)
	fmt.Fprintf(w, "  Network       %s\n", r.Network)
	fmt.Fprintf(w, "  Deployer      %s\n", r.Deployer)
	if r.MinResourceFee != 0 {
		fmt.Fprintf(w, "  Resource fee  %d stroops\n", r.MinResourceFee)
	}
	if r.UploadTxHash != "" {
		fmt.Fprintf(w, "  Upload tx     %s\n", r.UploadTxHash)
	}
	if r.Ledger != 0 {
		fmt.Fprintf(w, "  Ledger        %d\n", r.Ledger)
	}

	if r.DryRun && r.UploadEnvelopeXDR != "" {
		fmt.Fprintf(w, "\nAssembled upload envelope (unsigned):\n\n  %s\n", r.UploadEnvelopeXDR)
		fmt.Fprintf(w, "\nRe-run without --dry-run to submit.\n")
	}

	return nil
}

func printExtendResult(r *deploy.ExtendResult) error {
	if flags.jsonOutput {
		return printJSON(r)
	}

	w := out()
	if r.DryRun {
		fmt.Fprintf(w, "Dry run — nothing was submitted and nothing was recorded.\n\n")
	} else {
		fmt.Fprintf(w, "Extended %s's TTLs on %s.\n\n", r.Alias, r.Network)
	}

	fmt.Fprintf(w, "  Contract ID   %s\n", r.ContractID)
	fmt.Fprintf(w, "  WASM hash     %s\n", r.WasmHash)
	fmt.Fprintf(w, "  Entries       %s\n", strings.Join(r.Entries, ", "))
	fmt.Fprintf(w, "  Extended      %d ledgers (%s)\n", r.ExtendedLedgers, describeLedgers(r.ExtendedLedgers))
	if r.Capped {
		fmt.Fprintf(w, "                capped from %d at the network's maximum of %d\n",
			r.RequestedLedgers, r.MaxEntryTTL-1)
	}
	fmt.Fprintf(w, "  Deployer      %s\n", r.Deployer)
	if r.MinResourceFee != 0 {
		fmt.Fprintf(w, "  Resource fee  %d stroops\n", r.MinResourceFee)
	}
	if r.TxHash != "" {
		fmt.Fprintf(w, "  Extend tx     %s\n", r.TxHash)
	}
	if r.Ledger != 0 {
		fmt.Fprintf(w, "  Ledger        %d\n", r.Ledger)
	}

	if r.DryRun && r.EnvelopeXDR != "" {
		fmt.Fprintf(w, "\nAssembled extend envelope (unsigned):\n\n  %s\n", r.EnvelopeXDR)
		fmt.Fprintf(w, "\nRe-run without --dry-run to submit.\n")
	}

	return nil
}

func printRestoreResult(r *deploy.RestoreResult) error {
	if flags.jsonOutput {
		return printJSON(r)
	}

	w := out()
	if r.NothingToRestore {
		fmt.Fprintf(w, "Nothing to restore — %s's entries are already live on %s.\n\n", r.Alias, r.Network)
		printRestoreFields(w, r)
		return nil
	}

	if r.DryRun {
		fmt.Fprintf(w, "Dry run — nothing was submitted and nothing was recorded.\n\n")
	} else {
		fmt.Fprintf(w, "Restored %s on %s.\n\n", r.Alias, r.Network)
	}

	printRestoreFields(w, r)

	if r.DryRun && r.EnvelopeXDR != "" {
		fmt.Fprintf(w, "\nAssembled restore envelope (unsigned):\n\n  %s\n", r.EnvelopeXDR)
		fmt.Fprintf(w, "\nRe-run without --dry-run to submit.\n")
	}

	return nil
}

func printRestoreFields(w io.Writer, r *deploy.RestoreResult) {
	fmt.Fprintf(w, "  Contract ID   %s\n", r.ContractID)
	fmt.Fprintf(w, "  WASM hash     %s\n", r.WasmHash)
	if len(r.Entries) > 0 {
		fmt.Fprintf(w, "  Entries       %s\n", strings.Join(r.Entries, ", "))
	}
	fmt.Fprintf(w, "  Deployer      %s\n", r.Deployer)
	if r.MinResourceFee != 0 {
		fmt.Fprintf(w, "  Resource fee  %d stroops\n", r.MinResourceFee)
	}
	if r.TxHash != "" {
		fmt.Fprintf(w, "  Restore tx    %s\n", r.TxHash)
	}
	if r.Ledger != 0 {
		fmt.Fprintf(w, "  Ledger        %d\n", r.Ledger)
	}
}

func printFundResult(r *deploy.FundResult) error {
	if flags.jsonOutput {
		return printJSON(r)
	}

	w := out()
	if r.AlreadyFunded {
		fmt.Fprintf(w, "%s already has its starting balance on %s; nothing to do.\n\n",
			r.Address, r.Network)
	} else {
		fmt.Fprintf(w, "Funded %s on %s.\n\n", r.Address, r.Network)
	}

	fmt.Fprintf(w, "  Address       %s\n", r.Address)
	fmt.Fprintf(w, "  Network       %s\n", r.Network)
	fmt.Fprintf(w, "  Friendbot     %s\n", r.FriendbotURL)
	if r.Hash != "" {
		fmt.Fprintf(w, "  Funding tx    %s\n", r.Hash)
	}
	return nil
}

func printWhoAmIResult(r *deploy.WhoAmIResult) error {
	if flags.jsonOutput {
		return printJSON(r)
	}

	w := out()
	fmt.Fprintf(w, "%s\n\n", r.Detail)

	fmt.Fprintf(w, "  Address       %s\n", r.Address)
	if r.KeySource != "" {
		fmt.Fprintf(w, "  Key source    %s\n", r.KeySource)
	}
	fmt.Fprintf(w, "  Network       %s\n", r.Network)
	if r.Funded {
		fmt.Fprintf(w, "  Balance       %s XLM\n", r.BalanceXLM)
		fmt.Fprintf(w, "  Sequence      %d\n", r.Sequence)
	} else {
		fmt.Fprintf(w, "  Funded        no (the account has no ledger entry yet)\n")
		if r.FriendbotURL != "" {
			fmt.Fprintf(w, "  Friendbot     %s\n", r.FriendbotURL)
		}
	}
	return nil
}

// printExtension reports an --extend-ttl follow-up. A failure is a warning,
// not an error: the deploy or upgrade itself succeeded and is recorded.
func printExtension(w io.Writer, alias string, ext *deploy.ExtendResult, extErr string) {
	switch {
	case extErr != "":
		fmt.Fprintf(w, "  TTL           not extended: %s\n", extErr)
		fmt.Fprintf(w, "                (the contract is live; run `soroforge extend %s` to retry)\n", alias)
	case ext != nil:
		fmt.Fprintf(w, "  TTL           extended %d ledgers (tx %s)\n",
			ext.ExtendedLedgers, shortHash(ext.TxHash))
	}
}

// describeLedgers renders a TTL in ledgers as wall-clock time, at Stellar's
// ~5-second close time. The protocol counts in ledgers; operators think in
// days.
func describeLedgers(ledgers uint32) string {
	seconds := int64(ledgers) * 5
	switch {
	case seconds >= 48*3600:
		return fmt.Sprintf("about %.0f days", float64(seconds)/(24*3600))
	case seconds >= 2*3600:
		return fmt.Sprintf("about %.0f hours", float64(seconds)/3600)
	case seconds >= 120:
		return fmt.Sprintf("about %.0f minutes", float64(seconds)/60)
	default:
		return fmt.Sprintf("about %d seconds", seconds)
	}
}

// printCatalog reports SoroVault registration. A failure is shown as a
// warning, not an error: the deploy itself succeeded and is recorded.
func printCatalog(w io.Writer, c *deploy.CatalogStatus) {
	if c == nil {
		return
	}
	if !c.OK {
		fmt.Fprintf(w, "  SoroVault     not registered: %s\n", c.Error)
		fmt.Fprintf(w, "                (the deploy succeeded; run `sorovault add` to catch up)\n")
		return
	}
	fmt.Fprintf(w, "  SoroVault     %s (%d functions)\n", c.Entry.URL, c.Entry.Functions)
}

func printContracts(contracts []store.Contract) error {
	if flags.jsonOutput {
		return printJSON(map[string]any{"contracts": contracts})
	}

	if len(contracts) == 0 {
		fmt.Fprintln(out(), "No contracts tracked yet. Run `soroforge deploy <alias>` to add one.")
		return nil
	}

	tw := tabwriter.NewWriter(out(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NETWORK\tALIAS\tCONTRACT ID\tWASM HASH\tDEPLOYED")
	for _, c := range contracts {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			c.Network, c.Alias, c.ContractID, shortHash(c.CurrentWasmHash), formatTime(c.UpdatedAt))
	}
	return tw.Flush()
}

func printHistory(alias, network string, deployments []store.Deployment) error {
	if flags.jsonOutput {
		return printJSON(map[string]any{
			"alias":       alias,
			"network":     network,
			"deployments": deployments,
		})
	}

	if len(deployments) == 0 {
		fmt.Fprintf(out(), "No history for %s on %s.\n", alias, network)
		return nil
	}

	fmt.Fprintf(out(), "History for %s on %s (newest first):\n\n", alias, network)

	tw := tabwriter.NewWriter(out(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "WHEN\tACTION\tWASM HASH\tDEPLOYER\tLEDGER\tTX\tNOTES")
	for _, d := range deployments {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			formatTime(d.CreatedAt),
			d.Action,
			shortHash(d.WasmHash),
			shortHash(d.DeployerPubkey),
			formatLedger(d.Ledger),
			shortHash(d.TxHash),
			truncate(d.Notes, 40),
		)
	}
	return tw.Flush()
}

func printStatus(r *deploy.StatusResult) error {
	if flags.jsonOutput {
		return printJSON(r)
	}

	w := out()
	fmt.Fprintf(w, "%s\n\n", r.Detail)

	fmt.Fprintf(w, "  Alias         %s\n", r.Alias)
	fmt.Fprintf(w, "  Network       %s\n", r.Network)
	if r.ContractID != "" {
		fmt.Fprintf(w, "  Contract ID   %s\n", r.ContractID)
	}
	fmt.Fprintf(w, "  State         %s\n", r.State)
	if r.ExpectedWasmHash != "" {
		fmt.Fprintf(w, "  Recorded      %s\n", r.ExpectedWasmHash)
	}
	if r.ActualWasmHash != "" {
		fmt.Fprintf(w, "  On-chain      %s\n", r.ActualWasmHash)
	}

	return nil
}

func printNetworkStatus(r *deploy.NetworkStatus) error {
	if flags.jsonOutput {
		return printJSON(r)
	}

	w := out()
	if len(r.Contracts) == 0 {
		fmt.Fprintf(w, "No contracts tracked on %s.\n", r.Network)
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ALIAS\tSTATE\tCONTRACT ID\tRECORDED\tON-CHAIN")
	for _, c := range r.Contracts {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			c.Alias, c.State, c.ContractID, shortHash(c.ExpectedWasmHash), shortHash(c.ActualWasmHash))
	}
	if err := tw.Flush(); err != nil {
		return err
	}

	synced := 0
	for _, c := range r.Contracts {
		if c.InSync() {
			synced++
		}
	}
	fmt.Fprintf(w, "\n%d of %d contract(s) on %s in sync.\n", synced, len(r.Contracts), r.Network)
	return nil
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.UTC().Format("2006-01-02 15:04 UTC")
}

func formatLedger(ledger uint32) string {
	if ledger == 0 {
		return "-"
	}
	return fmt.Sprintf("%d", ledger)
}

func truncate(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if s == "" {
		return "-"
	}
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
