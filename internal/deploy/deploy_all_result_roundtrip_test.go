package deploy_test

import (
	"encoding/json"
	"testing"

	"github.com/soroworks/soroforge/internal/deploy"
	"github.com/stretchr/testify/require"
)

func TestDeployAllResultJSONRoundTrip(t *testing.T) {
	original := deploy.DeployAllResult{
		Alias:  "my_contract",
		Status: deploy.DeployAllDeployed,
		Result: &deploy.DeployResult{
			Alias:        "my_contract",
			Network:      "testnet",
			ContractID:   "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABSC4",
			WasmHash:     "abc123def456",
			Deployer:     "GDQP2KPQGKIHYJGXNUIYOMHARUARCA7DJT5FO2FFOOKY3B2WSQHG4W37",
			UploadTxHash: "txhash1",
			CreateTxHash: "txhash2",
			Ledger:       1234567,
			DryRun:       false,
		},
		Error: "",
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var got deploy.DeployAllResult
	require.NoError(t, json.Unmarshal(data, &got))

	require.Equal(t, original.Alias, got.Alias)
	require.Equal(t, original.Status, got.Status)
	require.Equal(t, original.Error, got.Error)
	require.NotNil(t, got.Result)
	require.Equal(t, *original.Result, *got.Result)
}

func TestDeployAllResultJSONRoundTripFailed(t *testing.T) {
	original := deploy.DeployAllResult{
		Alias:  "bad_contract",
		Status: deploy.DeployAllFailed,
		Result: nil,
		Error:  "simulation returned error: host function failure",
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var got deploy.DeployAllResult
	require.NoError(t, json.Unmarshal(data, &got))

	require.Equal(t, original, got)
}

func TestDeployAllResultJSONRoundTripSkipped(t *testing.T) {
	original := deploy.DeployAllResult{
		Alias:  "existing_contract",
		Status: deploy.DeployAllSkipped,
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var got deploy.DeployAllResult
	require.NoError(t, json.Unmarshal(data, &got))

	require.Equal(t, original, got)
}
