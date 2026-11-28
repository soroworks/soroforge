package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const readerContractID = "CA3D5KRYM6CB7OWQ6TWYRR3Z4T7GNZLKERYNZGGA5SOAOPIFY6YQGAXE"

// TestEnumJSONRoundTrip verifies that Enum marshals and unmarshals without data
// loss, including a non-empty Cases slice (#370).
func TestEnumJSONRoundTrip(t *testing.T) {
	orig := Enum{
		Name:  "Status",
		Cases: []EnumCase{{Name: "Active", Value: 1}, {Name: "Inactive", Value: 2}},
	}
	data, err := json.Marshal(orig)
	require.NoError(t, err)
	var got Enum
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, orig, got)
}

// TestEnumCaseJSONRoundTrip verifies that EnumCase marshals and unmarshals
// without data loss (#371).
func TestEnumCaseJSONRoundTrip(t *testing.T) {
	orig := EnumCase{Name: "NotFound", Value: 404}
	data, err := json.Marshal(orig)
	require.NoError(t, err)
	var got EnumCase
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, orig, got)
}

// TestEventJSONRoundTrip verifies that Event marshals and unmarshals without
// data loss, including a non-empty Params slice (#372).
func TestEventJSONRoundTrip(t *testing.T) {
	orig := Event{
		Name: "transferred",
		Params: []Param{
			{Name: "from", Type: Type{Display: "Address"}},
			{Name: "amount", Type: Type{Display: "i128"}},
		},
	}
	data, err := json.Marshal(orig)
	require.NoError(t, err)
	var got Event
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, orig, got)
}

func TestSoroVaultVersions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/contracts/"+readerContractID+"/versions", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"contract_id":"` + readerContractID + `","network":"testnet","versions":[
			{"wasm_hash":"9c1e","decoded_at":"2026-10-02T00:00:00Z","current":true},
			{"wasm_hash":"3f2a","decoded_at":"2026-10-01T00:00:00Z","current":false}]}`))
	}))
	t.Cleanup(srv.Close)

	versions, err := NewSoroVault(nil).Versions(context.Background(), srv.URL+"/", readerContractID)
	require.NoError(t, err)
	require.Len(t, versions, 2)
	assert.Equal(t, "9c1e", versions[0].WasmHash)
	assert.True(t, versions[0].Current)
	assert.Equal(t, "3f2a", versions[1].WasmHash)
}

func TestSoroVaultInterface(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/contracts/"+readerContractID, r.URL.Path)
		assert.Equal(t, "3f2a", r.URL.Query().Get("wasm_hash"))
		_, _ = w.Write([]byte(`{"contract_id":"x","wasm_hash":"3f2a","current":false,"interface":{
			"abi_version":"1",
			"functions":[{"name":"increment","inputs":[{"name":"by","type":{"kind":"u32","display":"u32"}}],
				"outputs":[{"kind":"u32","display":"u32"}]}],
			"types":{"structs":[{"name":"Config","fields":[{"name":"admin","type":{"kind":"address","display":"Address"}}],"is_tuple":false}],
				"unions":[],"enums":[],"error_enums":[{"name":"Error","cases":[{"name":"Nope","value":1}]}]},
			"events":[{"name":"bumped","params":[{"name":"by","type":{"display":"u32"},"location":"data"}],"data_format":"map"}]}}`))
	}))
	t.Cleanup(srv.Close)

	in, err := NewSoroVault(nil).Interface(context.Background(), srv.URL, readerContractID, "3f2a")
	require.NoError(t, err)
	require.Len(t, in.Functions, 1)
	assert.Equal(t, "fn increment(by: u32) -> u32", FunctionSignature(in.Functions[0]))
	require.Len(t, in.Types.Structs, 1)
	assert.Equal(t, "Address", in.Types.Structs[0].Fields[0].Type.Display)
	require.Len(t, in.Types.ErrorEnums, 1)
	require.Len(t, in.Events, 1)
}

func TestSoroVaultInterfaceEdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		baseURL    func(base string) string
		contractID string
		wasmHash   string
		ctx        func() (context.Context, context.CancelFunc)
		handler    http.HandlerFunc
		check      func(t *testing.T, iface *Interface, err error)
	}{
		{
			name:       "empty interface payload decodes with empty fields",
			contractID: readerContractID,
			wasmHash:   "empty",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/api/contracts/"+readerContractID, r.URL.Path)
				assert.Equal(t, "empty", r.URL.Query().Get("wasm_hash"))
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"contract_id":"` + readerContractID + `","wasm_hash":"empty","current":false,"interface":{"functions":[],"types":{"structs":[],"unions":[],"enums":[],"error_enums":[]},"events":[]}}`))
			},
			check: func(t *testing.T, iface *Interface, err error) {
				require.NoError(t, err)
				require.NotNil(t, iface)
				assert.Empty(t, iface.Functions)
				assert.Empty(t, iface.Events)
				assert.Empty(t, iface.Types.Structs)
				assert.Empty(t, iface.Types.Unions)
				assert.Empty(t, iface.Types.Enums)
				assert.Empty(t, iface.Types.ErrorEnums)
			},
		},
		{
			name: "boundary baseURL with trailing slashes and escaped query wasm hash",
			baseURL: func(base string) string {
				return base + "///"
			},
			contractID: readerContractID,
			wasmHash:   "hash with spaces & symbols+v1",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/api/contracts/"+readerContractID, r.URL.Path)
				assert.Equal(t, "hash with spaces & symbols+v1", r.URL.Query().Get("wasm_hash"))
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"interface":{"functions":[{"name":"init","inputs":[],"outputs":[]}]}}`))
			},
			check: func(t *testing.T, iface *Interface, err error) {
				require.NoError(t, err)
				require.NotNil(t, iface)
				require.Len(t, iface.Functions, 1)
				assert.Equal(t, "init", iface.Functions[0].Name)
			},
		},
		{
			name:       "invalid input with canceled context returns error",
			contractID: readerContractID,
			wasmHash:   "3f2a",
			ctx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, cancel
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			check: func(t *testing.T, iface *Interface, err error) {
				require.Error(t, err)
				assert.Nil(t, iface)
				assert.ErrorContains(t, err, "context canceled")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.handler)
			t.Cleanup(srv.Close)

			baseURL := srv.URL
			if tc.baseURL != nil {
				baseURL = tc.baseURL(srv.URL)
			}

			ctx := context.Background()
			if tc.ctx != nil {
				var cancel context.CancelFunc
				ctx, cancel = tc.ctx()
				defer cancel()
			}

			sv := NewSoroVault(nil)
			iface, err := sv.Interface(ctx, baseURL, tc.contractID, tc.wasmHash)
			tc.check(t, iface, err)
		})
	}
}

func TestParamJSONRoundTrip(t *testing.T) {
	orig := Param{Name: "amount", Type: Type{Display: "u128"}}
	b, err := json.Marshal(orig)
	require.NoError(t, err)
	var got Param
	require.NoError(t, json.Unmarshal(b, &got))
	assert.Equal(t, orig, got)
}

func TestFunctionJSONRoundTrip(t *testing.T) {
	orig := Function{
		Name:    "transfer",
		Inputs:  []Param{{Name: "from", Type: Type{Display: "Address"}}, {Name: "amount", Type: Type{Display: "u128"}}},
		Outputs: []Type{{Display: "bool"}},
	}
	b, err := json.Marshal(orig)
	require.NoError(t, err)
	var got Function
	require.NoError(t, json.Unmarshal(b, &got))
	assert.Equal(t, orig, got)
}

func TestInterfaceJSONRoundTrip(t *testing.T) {
	orig := Interface{
		Functions: []Function{{Name: "increment", Inputs: []Param{{Name: "by", Type: Type{Display: "u32"}}}, Outputs: []Type{{Display: "u32"}}}},
		Types: Types{
			Structs:    []Struct{{Name: "Config", Fields: []Param{{Name: "admin", Type: Type{Display: "Address"}}}}},
			Unions:     []Union{{Name: "Op", Cases: []UnionCase{{Name: "Add", Values: []Type{{Display: "u32"}}}}}},
			Enums:      []Enum{{Name: "Status", Cases: []EnumCase{{Name: "Active", Value: 1}}}},
			ErrorEnums: []Enum{{Name: "Error", Cases: []EnumCase{{Name: "Nope", Value: 1}}}},
		},
		Events: []Event{{Name: "bumped", Params: []Param{{Name: "by", Type: Type{Display: "u32"}}}}},
	}
	b, err := json.Marshal(orig)
	require.NoError(t, err)
	var got Interface
	require.NoError(t, json.Unmarshal(b, &got))
	assert.Equal(t, orig, got)
}

func TestSoroVaultReaderErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("wasm_hash") {
		case "nointerface":
			_, _ = w.Write([]byte(`{"contract_id":"x"}`))
		case "garbage":
			_, _ = w.Write([]byte(`not json`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"contract not found"}`))
		}
	}))
	t.Cleanup(srv.Close)
	sv := NewSoroVault(nil)

	_, err := sv.Versions(context.Background(), srv.URL, readerContractID)
	assert.EqualError(t, err, "sorovault: contract not found")

	_, err = sv.Interface(context.Background(), srv.URL, readerContractID, "nointerface")
	assert.ErrorContains(t, err, "no interface recorded")

	_, err = sv.Interface(context.Background(), srv.URL, readerContractID, "garbage")
	assert.ErrorContains(t, err, "decoding response")

	_, err = sv.Versions(context.Background(), "http://127.0.0.1:1", readerContractID)
	assert.ErrorContains(t, err, "sorovault:")
}

// TestStructJSONRoundTrip verifies that Struct marshals to JSON and back
// without loss (#377).
func TestStructJSONRoundTrip(t *testing.T) {
	t.Parallel()
	original := Struct{
		Name: "Config",
		Fields: []Param{
			{Name: "admin", Type: Type{Display: "Address"}},
			{Name: "fee", Type: Type{Display: "u32"}},
		},
	}
	data, err := json.Marshal(original)
	require.NoError(t, err)

	var got Struct
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, original, got)
}

// TestTypeJSONRoundTrip verifies that Type marshals to JSON and back
// without loss (#378).
func TestTypeJSONRoundTrip(t *testing.T) {
	t.Parallel()
	original := Type{Display: "Map<Symbol, i128>"}
	data, err := json.Marshal(original)
	require.NoError(t, err)

	var got Type
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, original, got)
}

// TestUnionCaseJSONRoundTrip verifies that UnionCase marshals to JSON and back
// without data loss, including a non-empty Values slice (#381).
func TestUnionCaseJSONRoundTrip(t *testing.T) {
	t.Parallel()
	original := UnionCase{
		Name:   "Deposit",
		Values: []Type{{Display: "Address"}, {Display: "u128"}},
	}
	data, err := json.Marshal(original)
	require.NoError(t, err)

	var got UnionCase
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, original, got)
}

// TestVersionJSONRoundTrip verifies that Version marshals to JSON and back
// without data loss, including the time.Time field (#382).
func TestVersionJSONRoundTrip(t *testing.T) {
	t.Parallel()
	original := Version{
		WasmHash:  "9c1e4f2a",
		DecodedAt: time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC),
		Current:   true,
	}
	data, err := json.Marshal(original)
	require.NoError(t, err)

	var got Version
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, original, got)
}

// TestUnionJSONRoundTrip verifies that Union marshals to JSON and back
// without loss (#380).
func TestUnionJSONRoundTrip(t *testing.T) {
	t.Parallel()
	original := Union{
		Name: "DataKey",
		Cases: []UnionCase{
			{Name: "Balance", Values: []Type{{Display: "Address"}, {Display: "u32"}}},
			{Name: "Allowance", Values: []Type{{Display: "AllowanceDataKey"}}},
			{Name: "Nonce", Values: []Type{}},
		},
	}
	data, err := json.Marshal(original)
	require.NoError(t, err)

	var got Union
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, original, got)
}
