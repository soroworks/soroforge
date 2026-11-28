package deploy

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/soroworks/soroforge/internal/store"
)

func TestWebhookStatusJSONRoundTrip(t *testing.T) {
	want := WebhookStatus{
		URL:      "https://hooks.example.test/deploy",
		OK:       false,
		Error:    "receiver timed out",
		Attempts: 3,
	}

	raw, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal WebhookStatus: %v", err)
	}

	var got WebhookStatus
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal WebhookStatus: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("WebhookStatus round trip = %#v, want %#v", got, want)
	}
}

func TestWebhookPayloadJSONRoundTrip(t *testing.T) {
	want := WebhookPayload{
		Alias:          "payments",
		Network:        "testnet",
		ContractID:     "CABC123",
		Contract:       "payments-v1",
		WasmHash:       "deadbeef",
		Action:         store.ActionUpgrade,
		DeployerPubkey: "GABC123",
		TxHash:         "0123456789abcdef",
		Tx:             "AAAA",
		Ledger:         987654,
		Notes:          "upgrade completed",
	}

	raw, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal WebhookPayload: %v", err)
	}

	var got WebhookPayload
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal WebhookPayload: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("WebhookPayload round trip = %#v, want %#v", got, want)
	}
}
