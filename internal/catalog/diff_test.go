package catalog

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fn(name string, out string, inputs ...Param) Function {
	f := Function{Name: name, Inputs: inputs}
	if out != "" {
		f.Outputs = []Type{{Display: out}}
	}
	return f
}

func p(name, typ string) Param { return Param{Name: name, Type: Type{Display: typ}} }

func TestDiff(t *testing.T) {
	base := Interface{
		Functions: []Function{fn("increment", "u32"), fn("get", "u32")},
		Types: Types{
			Structs: []Struct{{Name: "LegacyConfig", Fields: []Param{p("admin", "Address")}}},
			Enums:   []Enum{{Name: "Mode", Cases: []EnumCase{{"On", 1}}}},
		},
		Events: []Event{{Name: "incremented", Params: []Param{p("by", "u32")}}},
	}

	tests := []struct {
		name string
		to   Interface
		want []Change
	}{
		{
			name: "identical interfaces",
			to:   base,
			want: nil,
		},
		{
			name: "added function is not breaking",
			to: Interface{
				Functions: []Function{fn("increment", "u32"), fn("get", "u32"), fn("reset", "", p("admin", "Address"))},
				Types:     base.Types, Events: base.Events,
			},
			want: []Change{{Kind: Added, Item: "fn", Name: "reset", Signature: "fn reset(admin: Address)"}},
		},
		{
			name: "changed signature is breaking",
			to: Interface{
				Functions: []Function{fn("increment", "u32", p("by", "u32")), fn("get", "u32")},
				Types:     base.Types, Events: base.Events,
			},
			want: []Change{{
				Kind: Changed, Item: "fn", Name: "increment",
				Signature: "fn increment(by: u32) -> u32", Previous: "fn increment() -> u32", Breaking: true,
			}},
		},
		{
			name: "removed function and struct are breaking",
			to: Interface{
				Functions: []Function{fn("increment", "u32")},
				Types:     Types{Enums: base.Types.Enums}, Events: base.Events,
			},
			want: []Change{
				{Kind: Removed, Item: "fn", Name: "get", Signature: "fn get() -> u32", Breaking: true},
				{Kind: Removed, Item: "struct", Name: "LegacyConfig", Signature: "struct LegacyConfig { admin: Address }", Breaking: true},
			},
		},
		{
			name: "changed enum is breaking, changed event is not",
			to: Interface{
				Functions: base.Functions,
				Types: Types{
					Structs: base.Types.Structs,
					Enums:   []Enum{{Name: "Mode", Cases: []EnumCase{{"On", 1}, {"Off", 2}}}},
				},
				Events: []Event{{Name: "incremented", Params: []Param{p("by", "u64")}}},
			},
			want: []Change{
				{Kind: Changed, Item: "enum", Name: "Mode", Signature: "enum Mode { On = 1, Off = 2 }", Previous: "enum Mode { On = 1 }", Breaking: true},
				{Kind: Changed, Item: "event", Name: "incremented", Signature: "event incremented(by: u64)", Previous: "event incremented(by: u32)"},
			},
		},
		{
			name: "a type and a function may share a name",
			to: Interface{
				Functions: base.Functions,
				Types: Types{
					Structs: base.Types.Structs, Enums: base.Types.Enums,
					Unions:     []Union{{Name: "get", Cases: []UnionCase{{Name: "A", Values: []Type{{Display: "u32"}}}, {Name: "B"}}}},
					ErrorEnums: []Enum{{Name: "Error", Cases: []EnumCase{{"NotFound", 1}}}},
				},
				Events: base.Events,
			},
			want: []Change{
				{Kind: Added, Item: "union", Name: "get", Signature: "union get { A(u32), B }"},
				{Kind: Added, Item: "error", Name: "Error", Signature: "error Error { NotFound = 1 }"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Diff(base, tt.to)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, HasBreaking(tt.want), HasBreaking(got))
		})
	}
}

func TestDiffIsDeterministic(t *testing.T) {
	from := Interface{Functions: []Function{fn("a", ""), fn("b", ""), fn("c", "")}}
	to := Interface{Functions: []Function{fn("d", ""), fn("e", ""), fn("f", "")}}
	first := Diff(from, to)
	for i := 0; i < 20; i++ {
		assert.Equal(t, first, Diff(from, to))
	}
	assert.Len(t, first, 6)
	assert.Equal(t, "a", first[0].Name)
}

func TestFunctionSignature(t *testing.T) {
	assert.Equal(t, "fn noop()", FunctionSignature(fn("noop", "")))
	assert.Equal(t, "fn transfer(from: Address, to: Address, amount: i128) -> Result<(), Error>",
		FunctionSignature(fn("transfer", "Result<(), Error>", p("from", "Address"), p("to", "Address"), p("amount", "i128"))))
}

// TestChangeJSONRoundTrip verifies that Change marshals and unmarshals without
// data loss, including the omitempty Previous field (#369).
func TestChangeJSONRoundTrip(t *testing.T) {
	t.Run("all fields set", func(t *testing.T) {
		orig := Change{
			Kind:      Changed,
			Item:      "fn",
			Name:      "transfer",
			Signature: "fn transfer(to: Address, amount: i128) -> Result<(), Error>",
			Previous:  "fn transfer(to: Address)",
			Breaking:  true,
		}
		data, err := json.Marshal(orig)
		require.NoError(t, err)
		var got Change
		require.NoError(t, json.Unmarshal(data, &got))
		assert.Equal(t, orig, got)
	})

	t.Run("Previous omitted when empty", func(t *testing.T) {
		orig := Change{Kind: Added, Item: "fn", Name: "reset", Signature: "fn reset()"}
		data, err := json.Marshal(orig)
		require.NoError(t, err)
		assert.NotContains(t, string(data), "previous")
		var got Change
		require.NoError(t, json.Unmarshal(data, &got))
		assert.Equal(t, orig, got)
	})
}

func TestFunctionSignatureWithOnlyInputsOrOutputs(t *testing.T) {
	cases := []struct {
		name string
		fn   Function
		want string
	}{
		{
			name: "no inputs with a return value",
			fn:   fn("count", "u32"),
			want: "fn count() -> u32",
		},
		{
			name: "nested input type without a return value",
			fn:   fn("record", "", p("entries", "Vec<Option<Address>>")),
			want: "fn record(entries: Vec<Option<Address>>)",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, FunctionSignature(tc.fn))
		})
	}
}
