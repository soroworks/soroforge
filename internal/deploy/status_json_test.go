package deploy

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestVerifyRequestJSONRoundTrip(t *testing.T) {
	want := VerifyRequest{Alias: "token", Network: "testnet"}

	data, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal VerifyRequest: %v", err)
	}

	var got VerifyRequest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal VerifyRequest: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round trip = %+v, want %+v", got, want)
	}
}

func TestVerifyResultJSONRoundTrip(t *testing.T) {
	want := VerifyResult{
		Alias:         "token",
		Network:       "testnet",
		ContractID:    "CABC",
		Outcome:       VerifyMatch,
		WasmPath:      "target/wasm32v1-none/release/token.wasm",
		LocalWasmHash: "local-hash",
		LiveWasmHash:  "live-hash",
		Detail:        "the local build is live",
	}

	data, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal VerifyResult: %v", err)
	}

	var got VerifyResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal VerifyResult: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round trip = %+v, want %+v", got, want)
	}
}
