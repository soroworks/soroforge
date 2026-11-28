package config

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// UnmarshalJSON reads an --args-file value into the same shapes a
// soroforge.yaml value takes, keeping integers exact.
func TestValueUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		wantJSON string // what the value marshals back to
	}{
		{name: "integer", raw: `9`, wantJSON: `9`},
		// 2^53 + 1: a float64 decode would round this to 9007199254740992.
		{name: "integer beyond float64 precision", raw: `9007199254740993`, wantJSON: `9007199254740993`},
		{name: "integer beyond uint64 stays exact", raw: `340282366920938463463374607431768211455`, wantJSON: `"340282366920938463463374607431768211455"`},
		{name: "bool", raw: `true`, wantJSON: `true`},
		{name: "string", raw: `"GABC"`, wantJSON: `"GABC"`},
		{name: "list", raw: `[1, 2]`, wantJSON: `[1,2]`},
		{name: "object", raw: `{"key": "a"}`, wantJSON: `{"key":"a"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var v Value
			require.NoError(t, json.Unmarshal([]byte(tc.raw), &v))
			assert.True(t, v.set)
			out, err := json.Marshal(v)
			require.NoError(t, err)
			assert.JSONEq(t, tc.wantJSON, string(out))
		})
	}
}

// The exact integer reaches the encoder, which is the point of UseNumber.
func TestValueUnmarshalJSONKeepsLargeIntegersForToScVal(t *testing.T) {
	var arg Arg
	require.NoError(t, json.Unmarshal([]byte(`{"type": "u64", "value": 18446744073709551615}`), &arg))
	val, err := arg.ToScVal()
	require.NoError(t, err)
	require.NotNil(t, val.U64)
	assert.Equal(t, uint64(18446744073709551615), uint64(*val.U64))
}

func TestValueUnmarshalJSONNullLeavesValueUnset(t *testing.T) {
	for _, raw := range []string{`null`, ` null `} {
		var v Value
		require.NoError(t, v.UnmarshalJSON([]byte(raw)))
		assert.False(t, v.set, "%q must leave the value unset", raw)
	}
}

func TestValueUnmarshalJSONRejectsMalformedInput(t *testing.T) {
	for name, raw := range map[string]string{
		"truncated object": `{"key": `,
		"bare word":        `nope`,
		"trailing comma":   `[1, 2,]`,
	} {
		t.Run(name, func(t *testing.T) {
			var v Value
			assert.Error(t, v.UnmarshalJSON([]byte(raw)))
			assert.False(t, v.set, "a failed decode must not leave a value behind")
		})
	}
}
