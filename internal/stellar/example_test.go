package stellar_test

import (
	"fmt"

	"github.com/soroworks/soroforge/internal/stellar"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// ExampleContractIDFromSimulation recovers the contract ID a create-contract
// simulation reports, so a dry run can print the address a real deploy would
// produce before anything is submitted.
func ExampleContractIDFromSimulation() {
	// The simulated return value is the new contract's address, encoded as
	// base64 XDR. ContractIDFromSimulation decodes it back into a C... strkey.
	encoded, err := xdr.MarshalBase64(xdr.ScVal{
		Type: xdr.ScValTypeScvAddress,
		Address: &xdr.ScAddress{
			Type: xdr.ScAddressTypeScAddressTypeContract,
			ContractId: func() *xdr.ContractId {
				raw := make([]byte, 32)
				for i := range raw {
					raw[i] = byte(i + 1)
				}
				id := xdr.ContractId(raw)
				return &id
			}(),
		},
	})
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	got, err := stellar.ContractIDFromSimulation(stellar.SimulateResult{
		Results: []stellar.HostFunctionResult{{ReturnValueXDR: encoded}},
	})
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(got)

	// Output:
	// CAAQEAYEAUDAOCAJBIFQYDIOB4IBCEQTCQKRMFYYDENBWHA5DYPSBFLM
}
