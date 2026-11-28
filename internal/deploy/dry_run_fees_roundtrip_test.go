package deploy_test

import (
	"encoding/json"
	"testing"

	"github.com/soroworks/soroforge/internal/deploy"
	"github.com/stretchr/testify/require"
)

func TestDryRunFeesJSONRoundTrip(t *testing.T) {
	original := deploy.DryRunFees{
		Upload: &deploy.FeeEstimate{
			InclusionFeeStroops: 100,
			ResourceFeeStroops:  12345,
			TotalStroops:        12445,
			TotalXLM:            "0.0012445",
		},
		Create: &deploy.FeeEstimate{
			InclusionFeeStroops: 100,
			ResourceFeeStroops:  23456,
			TotalStroops:        23556,
			TotalXLM:            "0.0023556",
		},
		Upgrade: &deploy.FeeEstimate{
			InclusionFeeStroops: 100,
			ResourceFeeStroops:  34567,
			TotalStroops:        34667,
			TotalXLM:            "0.0034667",
		},
		Init: &deploy.FeeEstimate{
			InclusionFeeStroops: 100,
			ResourceFeeStroops:  45678,
			TotalStroops:        45778,
			TotalXLM:            "0.0045778",
		},
		Total: deploy.FeeEstimate{
			InclusionFeeStroops: 400,
			ResourceFeeStroops:  116046,
			TotalStroops:        116446,
			TotalXLM:            "0.0116446",
		},
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))
	for _, key := range []string{"upload", "create", "upgrade", "init", "total"} {
		require.Contains(t, raw, key)
	}

	var got deploy.DryRunFees
	require.NoError(t, json.Unmarshal(data, &got))
	require.Equal(t, original, got)
}

func TestDryRunFeesJSONRoundTripOmitEmpty(t *testing.T) {
	original := deploy.DryRunFees{
		Create: &deploy.FeeEstimate{
			InclusionFeeStroops: 100,
			ResourceFeeStroops:  12345,
			TotalStroops:        12445,
			TotalXLM:            "0.0012445",
		},
		Total: deploy.FeeEstimate{
			InclusionFeeStroops: 100,
			ResourceFeeStroops:  12345,
			TotalStroops:        12445,
			TotalXLM:            "0.0012445",
		},
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))
	require.NotContains(t, raw, "upload")
	require.NotContains(t, raw, "upgrade")
	require.NotContains(t, raw, "init")
	require.Contains(t, raw, "create")
	require.Contains(t, raw, "total")

	var got deploy.DryRunFees
	require.NoError(t, json.Unmarshal(data, &got))
	require.Equal(t, original, got)
}
