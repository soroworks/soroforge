package deploy

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestWhoAmIRequestJSONRoundTrip(t *testing.T) {
	want := WhoAmIRequest{Network: "testnet"}

	raw, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal WhoAmIRequest: %v", err)
	}

	var got WhoAmIRequest
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal WhoAmIRequest: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("WhoAmIRequest round trip = %#v, want %#v", got, want)
	}
}

func TestWhoAmIResultJSONRoundTrip(t *testing.T) {
	want := WhoAmIResult{
		Address:        "GABC123",
		KeySource:      "env:SORO_SECRET_KEY",
		Network:        "testnet",
		FriendbotURL:   "https://friendbot.example.test",
		Funded:         true,
		BalanceStroops: 12_345_678,
		BalanceXLM:     "1.2345678",
		Sequence:       42,
		Detail:         "GABC123 is funded on testnet",
	}

	raw, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal WhoAmIResult: %v", err)
	}

	var got WhoAmIResult
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal WhoAmIResult: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("WhoAmIResult round trip = %#v, want %#v", got, want)
	}
}
