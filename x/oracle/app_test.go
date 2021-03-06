// +build unit

package oracle_test

import (
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/mock"
	"github.com/dfinance/dvm-proto/go/vm_grpc"
	"github.com/stretchr/testify/require"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"

	"github.com/dfinance/dnode/x/oracle"
	"github.com/dfinance/dnode/x/oracle/internal/types"
)

const chainID = ""

type VMStorageImpl struct {
}

func NewVMStorage() VMStorageImpl {
	return VMStorageImpl{}
}

func (storage VMStorageImpl) GetOracleAccessPath(_ string) *vm_grpc.VMAccessPath {
	return &vm_grpc.VMAccessPath{}
}

func (storage VMStorageImpl) SetValue(ctx sdk.Context, accessPath *vm_grpc.VMAccessPath, value []byte) {
}

func (storage VMStorageImpl) GetValue(ctx sdk.Context, accessPath *vm_grpc.VMAccessPath) []byte {
	return nil
}

func (storage VMStorageImpl) HasValue(ctx sdk.Context, accessPath *vm_grpc.VMAccessPath) bool {
	return false
}

func (storage VMStorageImpl) DelValue(ctx sdk.Context, accessPath *vm_grpc.VMAccessPath) {
}

// GenTx generates a signed mock transaction.
func GenTx(msgs []sdk.Msg, accnums []uint64, seq []uint64, priv ...crypto.PrivKey) auth.StdTx {
	// Make the transaction free
	fee := auth.StdFee{
		Amount: sdk.NewCoins(sdk.NewInt64Coin("foocoin", 0)),
		Gas:    200000,
	}

	sigs := make([]auth.StdSignature, len(priv))
	memo := "testmemotestmemo"

	for i, p := range priv {
		sig, err := p.Sign(auth.StdSignBytes(chainID, accnums[i], seq[i], fee, msgs, memo))
		if err != nil {
			panic(err)
		}

		sigs[i] = auth.StdSignature{
			PubKey:    p.PubKey(),
			Signature: sig,
		}
	}

	return auth.NewStdTx(msgs, fee, sigs, memo)
}

// SignCheckDeliver checks a generated signed transaction and simulates a
// block commitment with the given transaction. A test assertion is made using
// the parameter 'expPass' against the result. A corresponding result is
// returned.
func SignCheckDeliver(
	t *testing.T, cdc *codec.Codec, app *baseapp.BaseApp, header abci.Header, msgs []sdk.Msg,
	accNums, seq []uint64, expSimPass, expPass bool, priv ...crypto.PrivKey,
) {

	tx := GenTx(msgs, accNums, seq, priv...)

	txBytes, err := cdc.MarshalBinaryLengthPrefixed(tx)
	require.Nil(t, err)

	// Must simulate now as CheckTx doesn't run Msgs anymore
	_, _, err = app.Simulate(txBytes, tx)
	if expSimPass {
		require.NoError(t, err)
	} else {
		require.Error(t, err)
	}

	// Simulate a sending a transaction and committing a block
	app.BeginBlock(abci.RequestBeginBlock{Header: header})
	_, _, err = app.Deliver(tx)
	if expPass {
		require.NoError(t, err)
	} else {
		require.Error(t, err)
	}

	app.EndBlock(abci.RequestEndBlock{})
	app.Commit()
}

func TestApp_PostPrice(t *testing.T) {
	// Setup
	mapp, keeper := setUpMockAppWithoutGenesis()
	genAccs, addrs, _, privKeys := mock.CreateGenAccounts(1, cs(c("uftm", 100)))
	testAddr := addrs[0]
	testPrivKey := privKeys[0]
	mock.SetGenesis(mapp, genAccs)
	// setup oracle, TODO can this be shortened a bit?
	header := abci.Header{Height: mapp.LastBlockHeight() + 1}
	mapp.BeginBlock(abci.RequestBeginBlock{Header: header})
	ctx := mapp.BaseApp.NewContext(false, header)
	oracleParams := oracle.DefaultParams()
	oracleParams.Assets = oracle.Assets{
		oracle.Asset{
			AssetCode: "uftm",
			Oracles: oracle.Oracles{
				oracle.Oracle{
					Address: addrs[0],
				},
			},
		},
	}
	oracleParams.Nominees = []string{addrs[0].String()}

	keeper.SetParams(ctx, oracleParams)
	_, _ = keeper.SetPrice(
		ctx, addrs[0], "uftm",
		sdk.NewInt(100000000),
		time.Now())
	_ = keeper.SetCurrentPrices(ctx)
	mapp.EndBlock(abci.RequestEndBlock{})
	mapp.Commit()

	layout := "2006-01-02T15:04:05.000Z"
	dateString := "2019-12-03T19:19:17.000Z"
	time1, _ := time.Parse(layout, dateString)

	// Create CSDT
	price, isOk := sdk.NewIntFromString("1")
	require.True(t, isOk)
	msgs := []sdk.Msg{types.NewMsgPostPrice(testAddr, "uftm", price, time1)}
	SignCheckDeliver(t, mapp.Cdc, mapp.BaseApp, abci.Header{Height: mapp.LastBlockHeight() + 1}, msgs, []uint64{0}, []uint64{0}, true, true, testPrivKey)
}

// Avoid cluttering test cases with long function name
func c(denom string, amount int64) sdk.Coin { return sdk.NewInt64Coin(denom, amount) }
func cs(coins ...sdk.Coin) sdk.Coins        { return sdk.NewCoins(coins...) }

func setUpMockAppWithoutGenesis() (*mock.App, oracle.Keeper) {
	// Create uninitialized mock app
	mapp := mock.NewApp()

	// Register codecs
	types.RegisterCodec(mapp.Cdc)

	// Create keepers
	keyOracle := sdk.NewKVStoreKey(oracle.StoreKey)

	// initialize vm keeper
	oracleKeeper := oracle.NewKeeper(keyOracle, mapp.Cdc, mapp.ParamsKeeper.Subspace(oracle.DefaultParamspace), NewVMStorage())

	// Register routes
	mapp.Router().AddRoute("oracle", oracle.NewHandler(oracleKeeper))
	// Mount and load the stores
	if err := mapp.CompleteSetup(keyOracle); err != nil {
		panic("mock app setup failed")
	}

	return mapp, oracleKeeper
}
