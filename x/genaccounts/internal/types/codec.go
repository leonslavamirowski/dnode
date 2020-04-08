package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
)

const ModuleName = "accounts"

// ModuleCdc - generic sealed codec to be used throughout this module
var ModuleCdc *codec.Codec

func init() {
	ModuleCdc = codec.New()
	codec.RegisterCrypto(ModuleCdc)
	ModuleCdc.Seal()
}
