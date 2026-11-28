package store_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/soroworks/soroforge/internal/store"
)

func TestSnapshotContractJSONRoundTrip(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	original := store.SnapshotContract{
		ID:              42,
		Alias:           "counter",
		Network:         "testnet",
		ContractID:      "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABSC4",
		CurrentWasmHash: "deadbeefdeadbeef",
		CreatedAt:       now,
		UpdatedAt:       now.Add(24 * time.Hour),
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got store.SnapshotContract
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.ID != original.ID {
		t.Errorf("ID: got %d, want %d", got.ID, original.ID)
	}
	if got.Alias != original.Alias {
		t.Errorf("Alias: got %q, want %q", got.Alias, original.Alias)
	}
	if got.Network != original.Network {
		t.Errorf("Network: got %q, want %q", got.Network, original.Network)
	}
	if got.ContractID != original.ContractID {
		t.Errorf("ContractID: got %q, want %q", got.ContractID, original.ContractID)
	}
	if got.CurrentWasmHash != original.CurrentWasmHash {
		t.Errorf("CurrentWasmHash: got %q, want %q", got.CurrentWasmHash, original.CurrentWasmHash)
	}
	if !got.CreatedAt.Equal(original.CreatedAt) {
		t.Errorf("CreatedAt: got %v, want %v", got.CreatedAt, original.CreatedAt)
	}
	if !got.UpdatedAt.Equal(original.UpdatedAt) {
		t.Errorf("UpdatedAt: got %v, want %v", got.UpdatedAt, original.UpdatedAt)
	}
}
