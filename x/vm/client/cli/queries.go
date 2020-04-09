package cli

import (
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/cosmos/cosmos-sdk/client/context"
	sdkClient "github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tendermint/go-amino"

	"github.com/dfinance/dvm-proto/go/vm_grpc"

	vmClient "github.com/dfinance/dnode/x/vm/client"
	"github.com/dfinance/dnode/x/vm/internal/types"
)

// Returns get commands for this module.
func GetQueryCmd(cdc *amino.Codec) *cobra.Command {
	queryCmd := &cobra.Command{
		Use:   types.ModuleName,
		Short: "VM query commands, includes compiler",
	}

	compileCommands := sdkClient.GetCommands(
		CompileScript(cdc),
		CompileModule(cdc),
	)
	for _, cmd := range compileCommands {
		cmd.Flags().String(vmClient.FlagCompilerAddr, vmClient.DefaultCompilerAddr, vmClient.FlagCompilerUsage)
		cmd.Flags().String(vmClient.FlagOutput, "", "--to-file ./compiled.mv")
	}

	commands := sdkClient.GetCommands(
		GetData(types.ModuleName, cdc),
	)
	commands = append(commands, compileCommands...)

	queryCmd.AddCommand(commands...)

	return queryCmd
}

// Read mvir file by file path.
func readMvirFile(filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return ioutil.ReadAll(file)
}

// Save output to stdout or file after compilation.
func saveOutput(bytecode []byte, cdc *codec.Codec) error {
	code := hex.EncodeToString(bytecode)
	output := viper.GetString(vmClient.FlagOutput)

	mvFile := vmClient.MVFile{Code: code}
	mvBytes, err := cdc.MarshalJSONIndent(mvFile, "", "    ")
	if err != nil {
		return err
	}

	if output == "" || output == "stdout" {
		fmt.Println("Compiled code: ")
		fmt.Println(string(mvBytes))
	} else {
		// write to file output
		if err := ioutil.WriteFile(output, mvBytes, 0644); err != nil {
			return err
		}

		fmt.Printf("Result saved to file %s\n", output)
	}

	return nil
}

// Get data from data source by access path.
func GetData(queryRoute string, cdc *codec.Codec) *cobra.Command {
	return &cobra.Command{
		Use:     "get-data [address] [path]",
		Short:   "get-data from data source storage by address and path, address could be bech32 or hex",
		Example: "get-data wallet1jk4ld0uu6wdrj9t8u3gghm9jt583hxx7xp7he8 0019b01c2cf3c2160a43e4dcad70e3e5d18151cc38de7a1d1067c6031bfa0ae4d9",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cliCtx := context.NewCLIContext().WithCodec(cdc)

			// extract data
			rawAddress := args[0]
			var address sdk.AccAddress
			address, err := hex.DecodeString(rawAddress)
			if err != nil {
				address, err = sdk.AccAddressFromBech32(rawAddress)
				if err != nil {
					return fmt.Errorf("can't parse address: %s\n, check address format, it could be libra hex or bech32", rawAddress)
				}

				address, err = hex.DecodeString(types.Bech32ToLibra(address))
				if err != nil {
					return fmt.Errorf("can't parse address: %s\n, check address format, it could be libra hex or bech32", rawAddress)
				}
			}

			path, err := hex.DecodeString(args[1])
			if err != nil {
				return err
			}

			bz, err := cdc.MarshalJSON(types.QueryAccessPath{
				Address: address,
				Path:    path,
			})
			if err != nil {
				return err
			}

			res, _, err := cliCtx.QueryWithData(
				fmt.Sprintf("custom/%s/value", queryRoute),
				bz)

			if err != nil {
				return err
			}

			out := types.QueryValueResp{Value: hex.EncodeToString(res)}

			return cliCtx.PrintOutput(out)
		},
	}
}

// Compile Mvir script.
func CompileScript(cdc *codec.Codec) *cobra.Command {
	return &cobra.Command{
		Use:     "compile-script [mvirFile] [account]",
		Short:   "compile script using source code from mvir file",
		Example: "compile-script script.mvir wallet196udj7s83uaw2u4safcrvgyqc0sc3flxuherp6 --to-file script.mvir.json",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			compilerAddr := viper.GetString(vmClient.FlagCompilerAddr)

			// read provided file
			mvirContent, err := readMvirFile(args[0])
			if err != nil {
				return fmt.Errorf("error during reading mvir file %q: %v", args[0], err)
			}

			// Mvir file
			sourceFile := &vm_grpc.MvIrSourceFile{
				Text:    string(mvirContent),
				Address: []byte(args[1]),
				Type:    vm_grpc.ContractType_Script,
			}

			// compile mvir file
			bytecode, err := vmClient.Compile(compilerAddr, sourceFile)
			if err != nil {
				return err
			}

			if err := saveOutput(bytecode, cdc); err != nil {
				return fmt.Errorf("error during compiled bytes output: %v", err)
			}

			fmt.Println("Compilation successful done.")

			return nil
		},
	}
}

// Compile Mvir module.
func CompileModule(cdc *codec.Codec) *cobra.Command {
	return &cobra.Command{
		Use:     "compile-module [mvirFile] [account]",
		Short:   "compile module connected to account, using source code from mvir file",
		Example: "compile-module module.mvir wallet196udj7s83uaw2u4safcrvgyqc0sc3flxuherp6 --to-file module.mvir.json",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			compilerAddr := viper.GetString(vmClient.FlagCompilerAddr)

			// read provided file
			mvirContent, err := readMvirFile(args[0])
			if err != nil {
				return fmt.Errorf("error during reading mvir file %q: %v", args[0], err)
			}

			// Mvir file
			sourceFile := &vm_grpc.MvIrSourceFile{
				Text:    string(mvirContent),
				Address: []byte(args[1]),
				Type:    vm_grpc.ContractType_Module,
			}

			// compile mvir file
			bytecode, err := vmClient.Compile(compilerAddr, sourceFile)
			if err != nil {
				return err
			}

			if err := saveOutput(bytecode, cdc); err != nil {
				return fmt.Errorf("error during compiled bytes output: %v", err)
			}

			fmt.Println("Compilation successful done.")

			return nil
		},
	}
}
