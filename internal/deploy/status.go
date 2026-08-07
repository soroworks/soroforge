package deploy

import (
	"context"
	"errors"
	"fmt"

	"github.com/soroworks/soroforge/internal/stellar"
	"github.com/soroworks/soroforge/internal/store"
)

// State is the outcome of comparing SoroForge's records against the ledger.
type State string

const (
	// StateInSync means the on-chain WASM hash matches what SoroForge recorded.
	StateInSync State = "in_sync"

	// StateDrift means the contract is running bytecode SoroForge did not
	// deploy. Someone upgraded it another way, or a deploy was recorded that
	// did not take effect.
	StateDrift State = "drift"

	// StateUntracked means the contract is not in SoroForge's records for this
	// network.
	StateUntracked State = "untracked"

	// StateMissing means SoroForge has a record but the ledger has no such
	// contract — it was never deployed to this network, or its instance entry
	// has expired.
	StateMissing State = "missing"
)

// StatusRequest asks for a drift check.
type StatusRequest struct {
	Alias   string `json:"alias"`
	Network string `json:"network,omitempty"`
}

// StatusResult reports what SoroForge believes against what the ledger says.
type StatusResult struct {
	Alias      string `json:"alias"`
	Network    string `json:"network"`
	ContractID string `json:"contract_id,omitempty"`

	// State summarises the comparison; the two hashes below are the evidence.
	State State `json:"state"`

	// ExpectedWasmHash is what SoroForge recorded as live.
	ExpectedWasmHash string `json:"expected_wasm_hash,omitempty"`

	// ActualWasmHash is what the ledger reports the contract is running.
	ActualWasmHash string `json:"actual_wasm_hash,omitempty"`

	// Detail explains the state in a sentence, for CLI output.
	Detail string `json:"detail"`
}

// InSync reports whether the contract matches its record. Callers use it to
// choose an exit code.
func (r StatusResult) InSync() bool { return r.State == StateInSync }

// Status compares the WASM hash SoroForge recorded against the one the contract
// is actually running.
//
// This is the check that makes the recorded history trustworthy. A database can
// drift from reality — someone deploys by hand, a migration is restored from a
// backup, an upgrade is applied with another tool — and a deployment tracker
// that never verifies itself will confidently report stale information. The
// ledger is the authority here; SoroForge's record is the claim being tested.
func (s *Service) Status(ctx context.Context, req StatusRequest) (*StatusResult, error) {
	r, err := s.resolve(req.Network, req.Alias)
	if err != nil {
		return nil, err
	}

	result := &StatusResult{
		Alias:   r.Alias,
		Network: r.NetworkName,
	}

	tracked, err := s.store.GetContract(ctx, r.NetworkName, r.Alias)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			result.State = StateUntracked
			result.Detail = fmt.Sprintf(
				"%s is not tracked on %s; SoroForge has no record of deploying it",
				r.Alias, r.NetworkName)
			return result, nil
		}
		return nil, err
	}

	result.ContractID = tracked.ContractID
	result.ExpectedWasmHash = tracked.CurrentWasmHash

	liveHash, err := stellar.OnChainWasmHash(ctx, r.Client, tracked.ContractID)
	if err != nil {
		var notFound *stellar.ErrContractNotFound
		if errors.As(err, &notFound) {
			result.State = StateMissing
			result.Detail = fmt.Sprintf(
				"SoroForge records %s as deployed at %s on %s, but that contract has no instance "+
					"on-chain (never deployed to this network, or its instance entry expired)",
				r.Alias, tracked.ContractID, r.NetworkName)
			return result, nil
		}
		return nil, err
	}

	result.ActualWasmHash = stellar.HashHex(liveHash)

	if result.ActualWasmHash == result.ExpectedWasmHash {
		result.State = StateInSync
		result.Detail = fmt.Sprintf("%s on %s matches its recorded wasm hash", r.Alias, r.NetworkName)
		return result, nil
	}

	result.State = StateDrift
	result.Detail = fmt.Sprintf(
		"drift: %s on %s is running %s but SoroForge recorded %s; "+
			"the contract was changed outside SoroForge",
		r.Alias, r.NetworkName, result.ActualWasmHash, result.ExpectedWasmHash)
	return result, nil
}

// List returns tracked contracts. An empty network name returns every network's
// contracts rather than defaulting to one, since `soroforge list` with no flags
// should show everything.
func (s *Service) List(ctx context.Context, network string) ([]store.Contract, error) {
	if network != "" {
		name, _, err := s.cfg.Network(network)
		if err != nil {
			return nil, err
		}
		network = name
	}
	return s.store.ListContracts(ctx, network)
}

// History returns a contract's deploy and upgrade log, newest first.
func (s *Service) History(ctx context.Context, network, alias string) ([]store.Deployment, error) {
	name, _, err := s.cfg.Network(network)
	if err != nil {
		return nil, err
	}
	return s.store.History(ctx, name, alias)
}
