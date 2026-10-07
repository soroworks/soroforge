// Package catalog publishes deployed contracts to a SoroVault interface
// registry, so a contract SoroForge deploys is immediately discoverable —
// its functions, types and events — without a separate registration step.
//
// Publishing is strictly best-effort. By the time it runs the contract is
// live on-chain and recorded in SoroForge's history; a registry being down
// must never turn a successful deploy into a reported failure.
package catalog

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultTimeout bounds one registration. SoroVault fetches and decodes the
// contract's code before answering, which is two RPC round trips.
const DefaultTimeout = 30 * time.Second

// maxResponseBytes caps how much of a response is read. A registration
// response carries the decoded interface, which is tens of kilobytes at most.
const maxResponseBytes = 4 << 20

// Registrar publishes a contract to an interface registry.
type Registrar interface {
	// Register asks the registry at baseURL to fetch, decode and record
	// contractID. It is idempotent: registering an upgraded contract records
	// its new interface version, and registering an unchanged one is a no-op.
	Register(ctx context.Context, baseURL, contractID string) (*Result, error)
}

// Result is what the registry recorded.
type Result struct {
	// URL is where the contract's interface can now be read.
	URL string `json:"url"`
	// Created is true when the registry had not seen the contract before.
	Created bool `json:"created"`
	// Changed is true when a new interface version was recorded.
	Changed bool `json:"changed"`
	// Functions is how many entry points the decoded interface exposes.
	Functions int `json:"functions"`
}

// SoroVault is a Registrar for SoroVault's JSON API.
type SoroVault struct {
	http *http.Client
}

var _ Registrar = (*SoroVault)(nil)

// NewSoroVault returns a Registrar using client, or an http.Client with
// DefaultTimeout when client is nil.
func NewSoroVault(client *http.Client) *SoroVault {
	if client == nil {
		client = &http.Client{Timeout: DefaultTimeout}
	}
	return &SoroVault{http: client}
}

// Register implements Registrar via POST /api/contracts.
func (s *SoroVault) Register(ctx context.Context, baseURL, contractID string) (*Result, error) {
	base := strings.TrimRight(baseURL, "/")

	body, err := json.Marshal(map[string]string{"contract_id": contractID})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/contracts", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("sorovault: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := s.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sorovault: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("sorovault: reading response: %w", err)
	}

	// 201 for a new contract, 200 for one already registered.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("sorovault: %s", errorMessage(raw, resp.Status))
	}

	var payload struct {
		Contract struct {
			Network string `json:"network"`
		} `json:"contract"`
		Created   bool `json:"created"`
		Changed   bool `json:"changed"`
		Interface *struct {
			Functions []json.RawMessage `json:"functions"`
		} `json:"interface"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("sorovault: decoding response: %w", err)
	}

	link := base + "/api/contracts/" + url.PathEscape(contractID)
	if payload.Contract.Network != "" {
		link += "?network=" + url.QueryEscape(payload.Contract.Network)
	}
	result := &Result{
		URL:     link,
		Created: payload.Created,
		Changed: payload.Changed,
	}
	if payload.Interface != nil {
		result.Functions = len(payload.Interface.Functions)
	}
	return result, nil
}

// errorMessage extracts SoroVault's {"error": "..."} body, falling back to
// the HTTP status.
func errorMessage(body []byte, status string) string {
	var e struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &e) == nil && e.Error != "" {
		return e.Error
	}
	return status
}
