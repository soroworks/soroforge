package catalog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const readerContractID = "CA3D5KRYM6CB7OWQ6TWYRR3Z4T7GNZLKERYNZGGA5SOAOPIFY6YQGAXE"

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
