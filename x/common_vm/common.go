package common_vm

import (
	"bytes"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/dfinance/dvm-proto/go/vm_grpc"
)

var (
	KeyDelimiter = []byte(":")
	VMKey        = []byte("vm")
)

// Interface for other keepers to get/set data.
type VMStorage interface {
	// Access path for oracle.
	GetOracleAccessPath(assetCode string) *vm_grpc.VMAccessPath

	// Setters/getters.
	SetValue(ctx sdk.Context, accessPath *vm_grpc.VMAccessPath, value []byte)
	GetValue(ctx sdk.Context, accessPath *vm_grpc.VMAccessPath) []byte

	// Delete value in VM storage.
	DelValue(ctx sdk.Context, accessPath * vm_grpc.VMAccessPath)
}

// Make path for storage from VMAccessPath.
func MakePathKey(path vm_grpc.VMAccessPath) []byte {
	return bytes.Join(
		[][]byte{
			VMKey,
			path.Address,
			path.Path,
		},
		KeyDelimiter,
	)
}

// Convert bech32 to libra hex.
func Bech32ToLibra(addr sdk.AccAddress) []byte {
	return append(addr, make([]byte, 4)...)
}
