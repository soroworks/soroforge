package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Reader reads the interfaces a registry has recorded for a contract, one per
// WASM version it has seen.
type Reader interface {
	// Versions lists the recorded versions of contractID, newest first.
	Versions(ctx context.Context, baseURL, contractID string) ([]Version, error)

	// Interface returns the decoded interface of one recorded version.
	Interface(ctx context.Context, baseURL, contractID, wasmHash string) (*Interface, error)
}

var _ Reader = (*SoroVault)(nil)

// Version is one recorded WASM version of a contract.
type Version struct {
	WasmHash  string    `json:"wasm_hash"`
	DecodedAt time.Time `json:"decoded_at"`
	// Current is true for the version the contract runs now.
	Current bool `json:"current"`
}

// Interface is the part of SoroVault's decoded contract interface that a diff
// compares: every name a caller can depend on, with enough type information
// to render a signature.
type Interface struct {
	Functions []Function `json:"functions"`
	Types     Types      `json:"types"`
	Events    []Event    `json:"events"`
}

// Function is a contract entry point.
type Function struct {
	Name    string  `json:"name"`
	Inputs  []Param `json:"inputs"`
	Outputs []Type  `json:"outputs"`
}

// Param is a named, typed value: a function input, struct field or event
// parameter.
type Param struct {
	Name string `json:"name"`
	Type Type   `json:"type"`
}

// Type is a Soroban type. SoroVault renders each one for display, and that
// rendering is what signatures are built from.
type Type struct {
	Display string `json:"display"`
}

// Types holds the user-defined types a contract declares.
type Types struct {
	Structs    []Struct `json:"structs"`
	Unions     []Union  `json:"unions"`
	Enums      []Enum   `json:"enums"`
	ErrorEnums []Enum   `json:"error_enums"`
}

// Struct is a contract struct.
type Struct struct {
	Name   string  `json:"name"`
	Fields []Param `json:"fields"`
}

// Union is a contract enum whose cases carry values.
type Union struct {
	Name  string      `json:"name"`
	Cases []UnionCase `json:"cases"`
}

// UnionCase is one case of a Union.
type UnionCase struct {
	Name   string `json:"name"`
	Values []Type `json:"values"`
}

// Enum is an integer enum, or an error enum.
type Enum struct {
	Name  string     `json:"name"`
	Cases []EnumCase `json:"cases"`
}

// EnumCase is one case of an Enum.
type EnumCase struct {
	Name  string `json:"name"`
	Value uint32 `json:"value"`
}

// Event is a contract event.
type Event struct {
	Name   string  `json:"name"`
	Params []Param `json:"params"`
}

// Versions implements Reader via GET /api/contracts/{id}/versions.
func (s *SoroVault) Versions(ctx context.Context, baseURL, contractID string) ([]Version, error) {
	var payload struct {
		Versions []Version `json:"versions"`
	}
	endpoint := strings.TrimRight(baseURL, "/") + "/api/contracts/" + url.PathEscape(contractID) + "/versions"
	if err := s.getJSON(ctx, endpoint, &payload); err != nil {
		return nil, err
	}
	return payload.Versions, nil
}

// Interface implements Reader via GET /api/contracts/{id}?wasm_hash=.
func (s *SoroVault) Interface(ctx context.Context, baseURL, contractID, wasmHash string) (*Interface, error) {
	var payload struct {
		Interface *Interface `json:"interface"`
	}
	endpoint := strings.TrimRight(baseURL, "/") + "/api/contracts/" + url.PathEscape(contractID) +
		"?wasm_hash=" + url.QueryEscape(wasmHash)
	if err := s.getJSON(ctx, endpoint, &payload); err != nil {
		return nil, err
	}
	if payload.Interface == nil {
		return nil, fmt.Errorf("sorovault: no interface recorded for %s at %s", contractID, wasmHash)
	}
	return payload.Interface, nil
}

// getJSON GETs endpoint and decodes a 200 response into out.
func (s *SoroVault) getJSON(ctx context.Context, endpoint string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("sorovault: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.http.Do(req)
	if err != nil {
		return fmt.Errorf("sorovault: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("sorovault: reading response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("sorovault: %s", errorMessage(raw, resp.Status))
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("sorovault: decoding response: %w", err)
	}
	return nil
}
