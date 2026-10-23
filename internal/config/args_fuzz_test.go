package config

import (
	"bytes"
	"testing"

	"github.com/stellar/go-stellar-sdk/xdr"
	"gopkg.in/yaml.v3"
)

// maxFuzzDepth bounds vec/map nesting. Deep inputs only re-exercise the same
// recursion, and an unbounded depth lets the fuzzer spend its time building
// enormous documents instead of exploring encoders.
const maxFuzzDepth = 16

// argSeeds gives every supported type a representative value, so the corpus
// starts inside each encoder rather than at its type-name check.
var argSeeds = map[string]string{
	ArgBool:      "true",
	ArgVoid:      "null",
	ArgU32:       "4294967295",
	ArgI32:       "-2147483648",
	ArgU64:       "18446744073709551615",
	ArgI64:       "-9223372036854775808",
	ArgU128:      `"340282366920938463463374607431768211455"`,
	ArgI128:      `"-170141183460469231731687303715884105728"`,
	ArgU256:      `"115792089237316195423570985008687907853269984665640564039457584007913129639935"`,
	ArgI256:      `"-1"`,
	ArgTimepoint: "1700000000",
	ArgDuration:  "3600",
	ArgString:    "hello",
	ArgSymbol:    "transfer",
	ArgBytes:     `"deadbeef"`,
	ArgAddress:   "GAAZI4TCR3TY5OJHCTJC2A4QSY6CJWJH5IAJTGKIN2ER7LBNVKOCCWN7",
	ArgVec:       "[{type: u32, value: 1}, {type: string, value: two}]",
	ArgMap:       "[{key: {type: symbol, value: a}, value: {type: u32, value: 1}}]",
	ArgXDR:       `"AAAAAQ=="`,
}

func yamlDepth(n *yaml.Node) int {
	deepest := 0
	for _, c := range n.Content {
		if d := yamlDepth(c); d > deepest {
			deepest = d
		}
	}
	return deepest + 1
}

// FuzzArgToScVal feeds arbitrary type names and YAML values through
// Arg.ToScVal. Errors are expected; panics are not. Anything that encodes must
// survive an XDR round trip unchanged.
func FuzzArgToScVal(f *testing.F) {
	for _, typ := range SupportedArgTypes {
		value, ok := argSeeds[typ]
		if !ok {
			value = "1"
		}
		f.Add(typ, []byte(value))
	}
	f.Add("", []byte("1"))
	f.Add("U32", []byte("7"))
	f.Add("vec", []byte("[[[[1]]]]"))
	f.Add("map", []byte("{a: b}"))
	f.Add("u128", []byte("0x10"))
	f.Add("bytes", []byte(`"zz"`))

	f.Fuzz(func(t *testing.T, typ string, raw []byte) {
		var node yaml.Node
		if err := yaml.Unmarshal(raw, &node); err != nil || len(node.Content) == 0 {
			return
		}
		if yamlDepth(&node) > maxFuzzDepth {
			return
		}

		arg := Arg{Type: typ, Value: Value{node: *node.Content[0], set: true}}
		val, err := arg.ToScVal()
		if err != nil {
			return
		}

		encoded, err := val.MarshalBinary()
		if err != nil {
			t.Fatalf("type %q encoded to an ScVal that does not marshal: %v", typ, err)
		}
		var back xdr.ScVal
		if err := back.UnmarshalBinary(encoded); err != nil {
			t.Fatalf("type %q: XDR does not unmarshal: %v", typ, err)
		}
		again, err := back.MarshalBinary()
		if err != nil {
			t.Fatalf("type %q: round-tripped ScVal does not marshal: %v", typ, err)
		}
		if !bytes.Equal(encoded, again) {
			t.Fatalf("type %q: XDR changed across a round trip", typ)
		}
	})
}
