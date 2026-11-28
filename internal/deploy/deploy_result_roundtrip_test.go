package deploy_test

import (
	"encoding/json"
	"testing"

	"github.com/soroworks/soroforge/internal/deploy"
)

func TestDeployResultJSONRoundTrip(t *testing.T) {
	original := deploy.DeployResult{
		Alias:             "counter",
		Network:           "testnet",
		ContractID:        "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABSC4",
		ContractURL:       "https://stellar.expert/explorer/testnet/contract/CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABSC4",
		WasmHash:          "deadbeefdeadbeef",
		Deployer:          "GAAZI4TCR3TY5OJHCTJC2A4QSY6CJWJH5IAJTGKIN2ER7LBNVKOCCWN",
		UploadTxHash:      "upload-tx-hash",
		CreateTxHash:      "create-tx-hash",
		UploadTxURL:       "https://stellar.expert/tx/upload",
		CreateTxURL:       "https://stellar.expert/tx/create",
		InitTxHash:        "init-tx-hash",
		Ledger:            12345,
		DryRun:            false,
		UploadEnvelopeXDR: "AAAA",
		CreateEnvelopeXDR: "BBBB",
		InitEnvelopeXDR:   "CCCC",
		UploadSkipped:     true,
		ConstructorArgs:   `[]`,
		Warnings:          []string{"missing contractspecv0"},
		ExtensionError:    "extend failed",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got deploy.DeployResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
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
	if got.WasmHash != original.WasmHash {
		t.Errorf("WasmHash: got %q, want %q", got.WasmHash, original.WasmHash)
	}
	if got.Deployer != original.Deployer {
		t.Errorf("Deployer: got %q, want %q", got.Deployer, original.Deployer)
	}
	if got.UploadTxHash != original.UploadTxHash {
		t.Errorf("UploadTxHash: got %q, want %q", got.UploadTxHash, original.UploadTxHash)
	}
	if got.CreateTxHash != original.CreateTxHash {
		t.Errorf("CreateTxHash: got %q, want %q", got.CreateTxHash, original.CreateTxHash)
	}
	if got.Ledger != original.Ledger {
		t.Errorf("Ledger: got %d, want %d", got.Ledger, original.Ledger)
	}
	if got.DryRun != original.DryRun {
		t.Errorf("DryRun: got %v, want %v", got.DryRun, original.DryRun)
	}
	if got.UploadSkipped != original.UploadSkipped {
		t.Errorf("UploadSkipped: got %v, want %v", got.UploadSkipped, original.UploadSkipped)
	}
	if got.ConstructorArgs != original.ConstructorArgs {
		t.Errorf("ConstructorArgs: got %q, want %q", got.ConstructorArgs, original.ConstructorArgs)
	}
	if got.ExtensionError != original.ExtensionError {
		t.Errorf("ExtensionError: got %q, want %q", got.ExtensionError, original.ExtensionError)
	}
	if len(got.Warnings) != len(original.Warnings) || got.Warnings[0] != original.Warnings[0] {
		t.Errorf("Warnings: got %v, want %v", got.Warnings, original.Warnings)
	}
}
