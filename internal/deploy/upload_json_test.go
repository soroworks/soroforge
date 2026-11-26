package deploy

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestUploadRequestJSONRoundTrip(t *testing.T) {
	want := UploadRequest{
		Alias:   "token",
		Network: "testnet",
		Notes:   "initial upload",
		DryRun:  true,
	}

	data, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal UploadRequest: %v", err)
	}

	var got UploadRequest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal UploadRequest: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round trip = %+v, want %+v", got, want)
	}
}

func TestUploadResultJSONRoundTrip(t *testing.T) {
	want := UploadResult{
		Alias:             "token",
		Network:           "testnet",
		WasmHash:          "wasm-hash",
		Deployer:          "GABC",
		UploadTxHash:      "tx-hash",
		Ledger:            42,
		MinResourceFee:    1234,
		DryRun:            true,
		UploadEnvelopeXDR: "envelope",
		UploadSkipped:     true,
	}

	data, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal UploadResult: %v", err)
	}

	var got UploadResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal UploadResult: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round trip = %+v, want %+v", got, want)
	}
}
