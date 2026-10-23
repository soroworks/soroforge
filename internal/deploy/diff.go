package deploy

import (
	"context"
	"errors"
	"fmt"

	"github.com/soroworks/soroforge/internal/catalog"
	"github.com/soroworks/soroforge/internal/store"
)

// DiffRequest asks for the interface change between two recorded versions of
// a contract. From and To are WASM hashes; either may be empty.
type DiffRequest struct {
	Alias   string `json:"alias"`
	Network string `json:"network,omitempty"`

	// From defaults to the version recorded before To.
	From string `json:"from,omitempty"`
	// To defaults to the contract's current version.
	To string `json:"to,omitempty"`
}

// DiffResult is the interface change an upgrade made.
type DiffResult struct {
	Alias      string `json:"alias"`
	Network    string `json:"network"`
	ContractID string `json:"contract_id"`
	Registry   string `json:"registry"`
	From       string `json:"from"`
	To         string `json:"to"`

	// Changes is empty, never null, when the interfaces match.
	Changes []catalog.Change `json:"changes"`

	// Breaking is true when any change can fail an existing caller.
	Breaking bool `json:"breaking"`
}

// Diff compares the interfaces SoroVault recorded for two versions of a
// tracked contract. By default it compares the last two recorded versions.
//
// The interfaces come from the network's sorovault_url: SoroForge records WASM
// hashes, but only the registry holds what each version's interface was.
func (s *Service) Diff(ctx context.Context, req DiffRequest) (*DiffResult, error) {
	name, network, err := s.cfg.Network(req.Network)
	if err != nil {
		return nil, err
	}
	if network.SoroVaultURL == "" {
		return nil, fmt.Errorf(
			"network %s has no sorovault_url: soroforge diff reads recorded interfaces from a SoroVault registry", name)
	}
	reader, ok := s.catalog.(catalog.Reader)
	if !ok {
		return nil, fmt.Errorf("the configured catalog cannot read recorded interfaces")
	}

	tracked, err := s.store.GetContract(ctx, name, req.Alias)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, fmt.Errorf("%s is not tracked on %s; there are no recorded versions to compare", req.Alias, name)
		}
		return nil, err
	}

	from, to := req.From, req.To
	if from == "" || to == "" {
		versions, err := reader.Versions(ctx, network.SoroVaultURL, tracked.ContractID)
		if err != nil {
			return nil, fmt.Errorf("list recorded versions of %s: %w", req.Alias, err)
		}
		from, to, err = pickVersions(versions, from, to)
		if err != nil {
			return nil, fmt.Errorf("%s on %s: %w", req.Alias, name, err)
		}
	}

	before, err := reader.Interface(ctx, network.SoroVaultURL, tracked.ContractID, from)
	if err != nil {
		return nil, fmt.Errorf("read interface of %s at %s: %w", req.Alias, from, err)
	}
	after, err := reader.Interface(ctx, network.SoroVaultURL, tracked.ContractID, to)
	if err != nil {
		return nil, fmt.Errorf("read interface of %s at %s: %w", req.Alias, to, err)
	}

	changes := catalog.Diff(*before, *after)
	if changes == nil {
		changes = []catalog.Change{}
	}
	return &DiffResult{
		Alias:      req.Alias,
		Network:    name,
		ContractID: tracked.ContractID,
		Registry:   network.SoroVaultURL,
		From:       from,
		To:         to,
		Changes:    changes,
		Breaking:   catalog.HasBreaking(changes),
	}, nil
}

// pickVersions fills in whichever of from and to is empty. versions is newest
// first: to defaults to the newest, and from to the version recorded just
// before to.
func pickVersions(versions []catalog.Version, from, to string) (string, string, error) {
	if len(versions) == 0 {
		return "", "", fmt.Errorf("the registry has no recorded versions")
	}
	if to == "" {
		to = versions[0].WasmHash
	}
	if from != "" {
		return from, to, nil
	}
	for i, v := range versions {
		if v.WasmHash != to {
			continue
		}
		if i+1 == len(versions) {
			return "", "", fmt.Errorf("no version is recorded before %s; pass --from to compare against another", to)
		}
		return versions[i+1].WasmHash, to, nil
	}
	return "", "", fmt.Errorf("version %s is not recorded in the registry", to)
}
