package deploy_test

import (
	"encoding/json"
	"testing"

	"github.com/soroworks/soroforge/internal/deploy"
)

func TestDiffRequestJSONRoundTrip(t *testing.T) {
	original := deploy.DiffRequest{
		Alias:   "counter",
		Network: "testnet",
		From:    "deadbeef01",
		To:      "deadbeef02",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got deploy.DiffRequest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Alias != original.Alias {
		t.Errorf("Alias: got %q, want %q", got.Alias, original.Alias)
	}
	if got.Network != original.Network {
		t.Errorf("Network: got %q, want %q", got.Network, original.Network)
	}
	if got.From != original.From {
		t.Errorf("From: got %q, want %q", got.From, original.From)
	}
	if got.To != original.To {
		t.Errorf("To: got %q, want %q", got.To, original.To)
	}
}
