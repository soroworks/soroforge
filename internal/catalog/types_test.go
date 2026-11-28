package catalog

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTypesJSONRoundTrip pins the JSON shape of Types: a value with one of
// every member must survive a marshal/unmarshal cycle unchanged, so the
// exported tags keep matching what the SoroVault API serves.
func TestTypesJSONRoundTrip(t *testing.T) {
	want := Types{
		Structs: []Struct{{
			Name: "Position",
			Fields: []Param{
				{Name: "x", Type: Type{Display: "i128"}},
				{Name: "y", Type: Type{Display: "i128"}},
			},
		}},
		Unions: []Union{{
			Name: "Asset",
			Cases: []UnionCase{
				{Name: "Native"},
				{Name: "Token", Values: []Type{{Display: "address"}}},
			},
		}},
		Enums: []Enum{{
			Name: "Side",
			Cases: []EnumCase{
				{Name: "Buy", Value: 0},
				{Name: "Sell", Value: 1},
			},
		}},
		ErrorEnums: []Enum{{
			Name:  "Failure",
			Cases: []EnumCase{{Name: "NotAuthorized", Value: 3}},
		}},
	}

	raw, err := json.Marshal(want)
	require.NoError(t, err)

	var got Types
	require.NoError(t, json.Unmarshal(raw, &got))

	assert.Equal(t, want, got)
}
