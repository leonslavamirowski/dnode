// Initialize commands for cli (add genesis validator).
package cli

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/server"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkErrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tendermint/tendermint/libs/cli"

	"github.com/dfinance/dnode/helpers"
	"github.com/dfinance/dnode/x/poa/types"
)

// Add PoA genesis validator via CLI to genesis.json.
func AddGenesisPoAValidatorCmd(ctx *server.Context, cdc *codec.Codec) *cobra.Command {
	return &cobra.Command{
		Use:   "add-genesis-poa-validator [address] [ethAddress]",
		Short: "Adds poa validator to genesis.json",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			config := ctx.Config
			config.SetRoot(viper.GetString(cli.HomeFlag))

			valAddr, err := sdk.AccAddressFromBech32(args[0])
			if err != nil {
				return fmt.Errorf("%s argument %q: %w", "address", args[0], err)
			}

			ethAddress := args[1]

			if !helpers.IsEthereumAddress(ethAddress) {
				return fmt.Errorf("%s argument %q is not an ethereum address", "ethAddress", ethAddress)
			}

			// retrieve the app state
			genFile := config.GenesisFile()
			appState, genDoc, err := genutil.GenesisStateFromGenFile(cdc, genFile)
			if err != nil {
				return err
			}

			var genesisState types.GenesisState
			cdc.MustUnmarshalJSON(appState[types.ModuleName], &genesisState)

			for _, val := range genesisState.PoAValidators {
				if val.Address.Equals(valAddr) || ethAddress == val.EthAddress {
					return fmt.Errorf("gensis file already contains validator with addresses %q / %q", val.Address.String(), val.EthAddress)
				}
			}

			if len(genesisState.PoAValidators) == int(genesisState.Parameters.MaxValidators) {
				return sdkErrors.Wrapf(types.ErrMaxValidatorsReached, "%d", genesisState.Parameters.MaxValidators)
			}

			validator := types.NewValidator(
				valAddr,
				ethAddress,
			)
			genesisState.PoAValidators = append(genesisState.PoAValidators, validator)

			genesisStateBz := cdc.MustMarshalJSON(genesisState)
			appState[types.ModuleName] = genesisStateBz

			appStateJson, err := cdc.MarshalJSON(appState)
			if err != nil {
				return err
			}

			genDoc.AppState = appStateJson

			return genutil.ExportGenesisFile(genDoc, genFile)
		},
	}
}
