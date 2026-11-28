package deploy_test

import (
	"encoding/json"
	"testing"

	"github.com/soroworks/soroforge/internal/deploy"
)

func TestDeployRequestJSONRoundTrip(t *testing.T) {
	original := deploy.DeployRequest{
		Alias:       "counter",
		Network:     "testnet",
		Notes:       "initial deploy",
		DryRun:      true,
		ExtendTTL:   true,
		GitCommit:   "abc1234",
		RequireSpec: true,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got deploy.DeployRequest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Alias != original.Alias {
		t.Errorf("Alias: got %q, want %q", got.Alias, original.Alias)
	}
	if got.Network != original.Network {
		t.Errorf("Network: got %q, want %q", got.Network, original.Network)
	}
	if got.Notes != original.Notes {
		t.Errorf("Notes: got %q, want %q", got.Notes, original.Notes)
	}
	if got.DryRun != original.DryRun {
		t.Errorf("DryRun: got %v, want %v", got.DryRun, original.DryRun)
	}
	if got.ExtendTTL != original.ExtendTTL {
		t.Errorf("ExtendTTL: got %v, want %v", got.ExtendTTL, original.ExtendTTL)
	}
	if got.GitCommit != original.GitCommit {
		t.Errorf("GitCommit: got %q, want %q", got.GitCommit, original.GitCommit)
	}
	if got.RequireSpec != original.RequireSpec {
		t.Errorf("RequireSpec: got %v, want %v", got.RequireSpec, original.RequireSpec)
	}
	// Fields tagged json:"-" must not survive a round trip.
	if got.Actor != "" {
		t.Errorf("Actor should not survive JSON round trip, got %q", got.Actor)
	}
	if got.ConstructorArgs != nil {
		t.Errorf("ConstructorArgs should not survive JSON round trip")
	}
	if got.InRunDeployments != nil {
		t.Errorf("InRunDeployments should not survive JSON round trip")
	}
}
