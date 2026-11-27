package config

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// valueDoc is the smallest document that carries a Value the way an Arg does.
type valueDoc struct {
	Value Value `yaml:"value"`
}

func TestValueYAMLRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{"int", "value: 7\n"},
		{"negative int", "value: -42\n"},
		{"beyond uint64", "value: 340282366920938463463374607431768211455\n"},
		{"string", "value: hello\n"},
		{"quoted number", "value: \"7\"\n"},
		{"bool", "value: true\n"},
		{"list with null", "value:\n    - null\n    - 1\n"},
		{"list", "value:\n    - 1\n    - 2\n    - 3\n"},
		{"nested list", "value:\n    - - a\n      - b\n    - - c\n"},
		{"list of pairs", "value:\n    - key: owner\n      value: GABC\n    - key: limit\n      value: 10\n"},
		{"mapping", "value:\n    type: u32\n    value: 5\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var first valueDoc
			require.NoError(t, yaml.Unmarshal([]byte(tt.yaml), &first))

			out, err := yaml.Marshal(first)
			require.NoError(t, err)

			var second valueDoc
			require.NoError(t, yaml.Unmarshal(out, &second))

			assert.Equal(t, first.Value.node.Kind, second.Value.node.Kind)
			assert.Equal(t, first.Value.node.Tag, second.Value.node.Tag)
			assert.Equal(t, first.Value.node.Value, second.Value.node.Value)

			// A second marshal is byte-identical: nothing drifts per pass.
			again, err := yaml.Marshal(second)
			require.NoError(t, err)
			assert.Equal(t, string(out), string(again))

			var a, b any
			require.NoError(t, first.Value.Decode(&a))
			require.NoError(t, second.Value.Decode(&b))
			assert.Equal(t, a, b)
		})
	}
}

func TestValueMarshalYAMLUnset(t *testing.T) {
	out, err := Value{}.MarshalYAML()
	require.NoError(t, err)
	assert.Nil(t, out, "an unset value marshals as null")

	encoded, err := yaml.Marshal(valueDoc{})
	require.NoError(t, err)
	assert.Equal(t, "value: null\n", string(encoded))
}

func TestKindName(t *testing.T) {
	tests := []struct {
		kind yaml.Kind
		want string
	}{
		{yaml.SequenceNode, "a list"},
		{yaml.MappingNode, "a mapping"},
		{yaml.AliasNode, "an alias"},
		{yaml.DocumentNode, "a document"},
		// kindName is only reached for non-scalar nodes (scalar() returns
		// early for those), so a scalar — including a nested null, which
		// yaml.v3 represents as a scalar tagged !!null — falls to the default.
		{yaml.ScalarNode, "an unknown node"},
		{yaml.Kind(0), "an unknown node"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, kindName(tt.kind))
	}
}

func TestScalarAccessorNamesNonScalarKind(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want string
	}{
		{"sequence", "value: [1, 2]\n", "got a list"},
		{"mapping", "value: {a: 1}\n", "got a mapping"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var doc valueDoc
			require.NoError(t, yaml.Unmarshal([]byte(tt.yaml), &doc))
			_, err := doc.Value.asString()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "expected a single value, "+tt.want)
		})
	}

	// yaml.v3 does not call UnmarshalYAML for a null, so `value: null` is the
	// same as no value at all; a null nested in a list is a !!null scalar.
	t.Run("null", func(t *testing.T) {
		var doc valueDoc
		require.NoError(t, yaml.Unmarshal([]byte("value: null\n"), &doc))
		assert.False(t, doc.Value.set)
		_, err := doc.Value.asString()
		assert.EqualError(t, err, "missing value")

		var list valueDoc
		require.NoError(t, yaml.Unmarshal([]byte("value: [null]\n"), &list))
		require.Len(t, list.Value.node.Content, 1)
		assert.Equal(t, yaml.ScalarNode, list.Value.node.Content[0].Kind)
		assert.Equal(t, "!!null", list.Value.node.Content[0].Tag)
	})

	t.Run("unset", func(t *testing.T) {
		_, err := Value{}.asString()
		assert.EqualError(t, err, "missing value")
	})
}

func TestNewValue(t *testing.T) {
	v := NewValue([]any{int64(1), "two", map[string]any{"k": true}})
	assert.True(t, v.set)
	assert.Equal(t, yaml.SequenceNode, v.node.Kind)

	n := NewValue(int64(-5))
	got, err := n.asInt(32)
	require.NoError(t, err)
	assert.Equal(t, int64(-5), got)

	assert.Panics(t, func() { NewValue(make(chan int)) },
		"a type yaml cannot represent is a programming error")
}

func TestAsIntEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		value any
		bits  int
		want  int64
		err   bool
	}{
		{"i32 max", int64(math.MaxInt32), 32, math.MaxInt32, false},
		{"i32 min", int64(math.MinInt32), 32, math.MinInt32, false},
		{"i32 overflow", int64(math.MaxInt32) + 1, 32, 0, true},
		{"i32 underflow", int64(math.MinInt32) - 1, 32, 0, true},
		{"i64 max", int64(math.MaxInt64), 64, math.MaxInt64, false},
		{"i64 overflow", "9223372036854775808", 64, 0, true},
		{"quoted", "12", 32, 12, false},
		{"not a number", "twelve", 32, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewValue(tt.value).asInt(tt.bits)
			if tt.err {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "is not a valid i"+strconv.Itoa(tt.bits))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAsUintRejectsNegativeAndOverflow(t *testing.T) {
	tests := []struct {
		name  string
		value any
		bits  int
		want  uint64
		err   bool
	}{
		{"u32 max", uint64(math.MaxUint32), 32, math.MaxUint32, false},
		{"u32 overflow", uint64(math.MaxUint32) + 1, 32, 0, true},
		{"u32 negative", int64(-1), 32, 0, true},
		{"u64 max", uint64(math.MaxUint64), 64, math.MaxUint64, false},
		{"u64 overflow", "18446744073709551616", 64, 0, true},
		{"u64 negative", int64(-1), 64, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewValue(tt.value).asUint(tt.bits)
			if tt.err {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "is not a valid u"+strconv.Itoa(tt.bits))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValueDecodeEdgeCases(t *testing.T) {
	tests := map[string]struct {
		yaml    string // document holding `value:`; empty means the field is absent
		decode  func(Value) error
		wantErr string
	}{
		"absent value": {
			yaml:    "",
			decode:  func(v Value) error { var s string; return v.Decode(&s) },
			wantErr: "missing value",
		},
		"uint64 boundary decodes exactly": {
			yaml: "value: 18446744073709551615",
			decode: func(v Value) error {
				var n uint64
				if err := v.Decode(&n); err != nil {
					return err
				}
				if n != math.MaxUint64 {
					return assert.AnError
				}
				return nil
			},
		},
		"list into a scalar is rejected": {
			yaml:    "value: [1, 2]",
			decode:  func(v Value) error { var n int; return v.Decode(&n) },
			wantErr: "cannot unmarshal",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var doc struct {
				Value Value `yaml:"value"`
			}
			require.NoError(t, yaml.Unmarshal([]byte(tc.yaml), &doc))

			err := tc.decode(doc.Value)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}

// ExampleNewValue shows the constructor used when an Arg is built in Go rather
// than decoded from YAML: the Value behaves exactly like one unmarshalled from
// a config file.
func ExampleNewValue() {
	arg := Arg{Type: ArgU32, Value: NewValue(7)}
	val, err := arg.ToScVal()
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(*val.U32)

	// NewValue keeps integers exact instead of routing them through float64,
	// which is what makes a u64 at the top of its range safe to pass.
	big := NewValue(uint64(math.MaxUint64))
	var n uint64
	if err := big.Decode(&n); err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(n)

	// Recording a deployment marshals the same Value as JSON, in the numeric
	// form the equivalent config file would have produced.
	recorded, err := json.Marshal(Arg{Type: ArgU64, Value: big})
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(string(recorded))

	// Output:
	// 7
	// 18446744073709551615
	// {"type":"u64","value":18446744073709551615}
}
