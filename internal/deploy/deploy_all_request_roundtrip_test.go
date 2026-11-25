package deploy_test

import (
	"encoding/json"
	"testing"

	"github.com/soroworks/soroforge/internal/deploy"
	"github.com/stretchr/testify/require"
)

func TestDeployAllRequestJSONRoundTrip(t *testing.T) {
	original := deploy.DeployAllRequest{
		Network: "testnet",
		Notes:   "automated nightly run",
		DryRun:  true,
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var got deploy.DeployAllRequest
	require.NoError(t, json.Unmarshal(data, &got))

	require.Equal(t, original, got)
}

func TestDeployAllRequestJSONRoundTripZero(t *testing.T) {
	original := deploy.DeployAllRequest{}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var got deploy.DeployAllRequest
	require.NoError(t, json.Unmarshal(data, &got))

	require.Equal(t, original, got)
}
