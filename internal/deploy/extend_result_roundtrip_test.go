package deploy_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/soroworks/soroforge/internal/deploy"
	"github.com/soroworks/soroforge/internal/store"
	"github.com/stretchr/testify/require"
)

func TestExtendResultJSONRoundTrip(t *testing.T) {
	fee := int64(12345)
	original := deploy.ExtendResult{
		Alias:            "counter",
		Network:          "testnet",
		ContractID:       "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABSC4",
		WasmHash:         "deadbeefdeadbeef",
		Deployer:         "GAAZI4TCR3TY5OJHCTJC2A4QSY6CJWJH5IAJTGKIN2ER7LBNVKOCCWN",
		RequestedLedgers: 500000,
		ExtendedLedgers:  400000,
		Capped:           true,
		MaxEntryTTL:      400000,
		Entries:          []string{"instance", "code"},
		MinResourceFee:   12345,
		TxHash:           "extend-tx-hash",
		Ledger:           98765,
		DryRun:           true,
		EnvelopeXDR:      "AAAA",
		Deployment: &store.Deployment{
			ID:             42,
			Alias:          "counter",
			Network:        "testnet",
			ContractID:     "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABSC4",
			WasmHash:       "deadbeefdeadbeef",
			Action:         store.ActionExtend,
			DeployerPubkey: "GAAZI4TCR3TY5OJHCTJC2A4QSY6CJWJH5IAJTGKIN2ER7LBNVKOCCWN",
			Actor:          "cli",
			TxHash:         "extend-tx-hash",
			Ledger:         98765,
			GitCommit:      "abcdef123456",
			Dirty:          false,
			FeeCharged:     &fee,
			Notes:          "monthly TTL renewal",
			CreatedAt:      time.Date(2026, 11, 28, 7, 40, 0, 0, time.UTC),
		},
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))
	for _, key := range []string{
		"alias",
		"network",
		"contract_id",
		"wasm_hash",
		"deployer_pubkey",
		"requested_ledgers",
		"extended_ledgers",
		"capped",
		"max_entry_ttl",
		"entries",
		"min_resource_fee",
		"tx_hash",
		"ledger",
		"dry_run",
		"envelope_xdr",
		"deployment",
	} {
		require.Contains(t, raw, key)
	}

	var got deploy.ExtendResult
	require.NoError(t, json.Unmarshal(data, &got))
	require.Equal(t, original, got)
}

func TestExtendResultJSONRoundTripOmitEmpty(t *testing.T) {
	original := deploy.ExtendResult{
		Alias:           "counter",
		Network:         "testnet",
		ContractID:      "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABSC4",
		WasmHash:        "deadbeefdeadbeef",
		Deployer:        "GAAZI4TCR3TY5OJHCTJC2A4QSY6CJWJH5IAJTGKIN2ER7LBNVKOCCWN",
		ExtendedLedgers: 100000,
		MaxEntryTTL:     400000,
		Entries:         []string{"instance", "code"},
		DryRun:          false,
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))
	for _, key := range []string{
		"requested_ledgers",
		"capped",
		"min_resource_fee",
		"tx_hash",
		"ledger",
		"envelope_xdr",
		"deployment",
	} {
		require.NotContains(t, raw, key)
	}

	var got deploy.ExtendResult
	require.NoError(t, json.Unmarshal(data, &got))
	require.Equal(t, original, got)
}
