// +build unit

package keeper

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"strconv"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/dfinance/dvm-proto/go/vm_grpc"

	"github.com/dfinance/dnode/x/common_vm"
	"github.com/dfinance/dnode/x/vm/internal/types"
)

// TODO: change listener logic to don't close it here?

// Check storage set value functional.
func TestSetValue(t *testing.T) {
	input := setupTestInput(true)
	defer closeInput(input)

	ap := &vm_grpc.VMAccessPath{
		Address: input.addressBytes,
		Path:    input.pathBytes,
	}

	input.vk.setValue(input.ctx, ap, input.valueBytes)
	value := input.vk.getValue(input.ctx, ap)

	require.True(t, bytes.Equal(input.valueBytes, value))

	isExists := input.vk.hasValue(input.ctx, ap)
	require.True(t, isExists)
}

// Check get value from storage functional.
func TestGetValue(t *testing.T) {
	input := setupTestInput(true)
	defer closeInput(input)

	ap := randomPath()
	input.vk.setValue(input.ctx, ap, input.valueBytes)

	value := input.vk.getValue(input.ctx, ap)
	require.Equal(t, input.valueBytes, value)

	notExistsPath := randomPath()

	var nilBytes []byte
	value = input.vk.getValue(input.ctx, notExistsPath)
	require.EqualValues(t, nilBytes, value)
}

// Check has value functional.
func TestHasValue(t *testing.T) {
	input := setupTestInput(true)
	defer closeInput(input)

	ap := randomPath()

	input.vk.setValue(input.ctx, ap, input.valueBytes)

	isExists := input.vk.hasValue(input.ctx, ap)
	require.True(t, isExists)

	wrongAp := randomPath()
	isExists = input.vk.hasValue(input.ctx, wrongAp)
	require.False(t, isExists)
}

// Check deletion of key in storage.
func TestDelValue(t *testing.T) {
	input := setupTestInput(true)
	defer closeInput(input)

	var emptyBytes []byte

	ap := randomPath()
	input.vk.setValue(input.ctx, ap, input.valueBytes)

	value := input.vk.getValue(input.ctx, ap)
	require.EqualValues(t, input.valueBytes, value)

	isExists := input.vk.hasValue(input.ctx, ap)
	require.True(t, isExists)

	input.vk.delValue(input.ctx, ap)

	value = input.vk.getValue(input.ctx, ap)
	require.EqualValues(t, emptyBytes, value)

	isExists = input.vk.hasValue(input.ctx, ap)
	require.False(t, isExists)
}

// Check process execution (response from VM) functional.
func TestProcessExecution(t *testing.T) {
	// ignoring gas for now.
	input := setupTestInput(true)
	defer closeInput(input)

	resp := &vm_grpc.VMExecuteResponse{
		Status: vm_grpc.ContractStatus_Discard,
		StatusStruct: &vm_grpc.VMStatus{
			MajorStatus: 1,
			SubStatus:   250,
			Message:     "this is another errorr!!!1111",
		},
	}

	input.vk.processExecution(input.ctx, resp)

	events := input.ctx.EventManager().Events()
	event := types.NewEventDiscard(resp.StatusStruct)

	require.Len(t, events, 1)
	require.Equal(t, event.Type, events[0].Type)
	require.Equal(t, event.Attributes, events[0].Attributes)

	// discard without status
	resp = &vm_grpc.VMExecuteResponse{
		Status: vm_grpc.ContractStatus_Discard,
	}

	ctx := input.ctx.WithEventManager(sdk.NewEventManager())
	input.vk.processExecution(ctx, resp)

	events = ctx.EventManager().Events()
	event = types.NewEventDiscard(nil)

	require.Len(t, events, 1)
	require.Equal(t, event, events[0])

	// status keep
	resp = &vm_grpc.VMExecuteResponse{
		Status: vm_grpc.ContractStatus_Keep,
	}

	ctx = input.ctx.WithEventManager(sdk.NewEventManager())
	input.vk.processExecution(ctx, resp)

	events = ctx.EventManager().Events()
	event = types.NewEventKeep()

	require.Len(t, events, 1)
	require.Equal(t, event, events[0])

	// write set & events
	var u64Value uint64 = 100
	u64Bytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(u64Bytes, u64Value)

	respEvents := make([]*vm_grpc.VMEvent, 2)
	respEvents[0] = &vm_grpc.VMEvent{
		Key:            []byte("test 1"),
		SequenceNumber: 0,
		Type: &vm_grpc.VMType{
			Tag: vm_grpc.VMTypeTag_ByteArray,
		},
		EventData: randomValue(32),
	}
	respEvents[1] = &vm_grpc.VMEvent{
		Key:            []byte("test 2"),
		SequenceNumber: 1,
		Type: &vm_grpc.VMType{
			Tag: vm_grpc.VMTypeTag_U64,
		},
		EventData: u64Bytes,
	}

	dnEvents := make(sdk.Events, 2)
	dnEvents[0] = types.NewEventFromVM(respEvents[0])
	dnEvents[1] = types.NewEventFromVM(respEvents[1])

	writeSet := make([]*vm_grpc.VMValue, 2)
	writeSet[0] = &vm_grpc.VMValue{
		Type:  vm_grpc.VmWriteOp_Value,
		Value: randomValue(32),
		Path:  randomPath(),
	}
	writeSet[1] = &vm_grpc.VMValue{
		Type:  vm_grpc.VmWriteOp_Value,
		Value: randomValue(16),
		Path:  randomPath(),
	}

	resp = &vm_grpc.VMExecuteResponse{
		WriteSet: writeSet,
		Events:   respEvents,
		Status:   vm_grpc.ContractStatus_Keep,
	}

	ctx = input.ctx.WithEventManager(sdk.NewEventManager())
	input.vk.processExecution(ctx, resp)
	events = ctx.EventManager().Events()

	// check that everything fine with write set
	for _, write := range writeSet {
		require.True(t, input.vk.hasValue(input.ctx, write.Path))
		require.Equal(t, write.Value, input.vk.getValue(input.ctx, write.Path))
	}

	require.Len(t, events, len(dnEvents)+1)

	for i, event := range events[1:] {
		require.Equal(t, dnEvents[i].Type, event.Type)

		for j, attr := range event.Attributes {
			require.EqualValues(t, dnEvents[i].Attributes[j].Key, attr.Key)
			require.EqualValues(t, dnEvents[i].Attributes[j].Value, attr.Value)
		}
	}

	// check deletion
	writeSet[1] = &vm_grpc.VMValue{
		Type: vm_grpc.VmWriteOp_Deletion,
		Path: writeSet[1].Path,
	}

	resp = &vm_grpc.VMExecuteResponse{
		WriteSet: writeSet,
		Status:   vm_grpc.ContractStatus_Keep,
	}

	ctx = input.ctx.WithEventManager(sdk.NewEventManager())
	input.vk.processExecution(ctx, resp)
	events = ctx.EventManager().Events()

	require.Len(t, events, 1)

	require.False(t, input.vk.hasValue(input.ctx, writeSet[1].Path))
	require.Nil(t, input.vk.getValue(input.ctx, writeSet[1].Path))
}

// Check returned write set procession.
func TestProcessWriteSet(t *testing.T) {
	input := setupTestInput(true)
	defer closeInput(input)

	writeSet := make([]*vm_grpc.VMValue, 2)
	writeSet[0] = &vm_grpc.VMValue{
		Type:  vm_grpc.VmWriteOp_Value,
		Value: randomValue(32),
		Path:  randomPath(),
	}
	writeSet[1] = &vm_grpc.VMValue{
		Type:  vm_grpc.VmWriteOp_Value,
		Value: randomValue(16),
		Path:  randomPath(),
	}

	input.vk.processWriteSet(input.ctx, writeSet)

	// now read storage and check results
	values := make([][]byte, 2)
	values[0] = input.vk.getValue(input.ctx, writeSet[0].Path)
	values[1] = input.vk.getValue(input.ctx, writeSet[1].Path)

	for i, write := range writeSet {
		require.True(t, input.vk.hasValue(input.ctx, write.Path))
		require.Equal(t, write.Value, values[i])
	}

	// check delete op
	delSet := make([]*vm_grpc.VMValue, 2)
	delSet[0] = &vm_grpc.VMValue{
		Type: vm_grpc.VmWriteOp_Deletion,
		Path: writeSet[0].Path,
	}
	delSet[1] = &vm_grpc.VMValue{
		Type: vm_grpc.VmWriteOp_Deletion,
		Path: writeSet[1].Path,
	}

	input.vk.processWriteSet(input.ctx, delSet)

	for _, del := range delSet {
		require.False(t, input.vk.hasValue(input.ctx, del.Path))
		value := input.vk.getValue(input.ctx, del.Path)
		require.Nil(t, value)
	}
}

// Status keep (4001) still doesn't contains error.
func TestExecStatusKeeperNotAnError(t *testing.T) {
	input := setupTestInput(true)
	defer closeInput(input)

	errorStatus := vm_grpc.VMStatus{
		MajorStatus: types.VMCodeExecuted,
		SubStatus:   0,
		Message:     "",
	}

	resp := &vm_grpc.VMExecuteResponse{
		WriteSet:     nil,
		Events:       nil,
		Status:       vm_grpc.ContractStatus_Keep,
		StatusStruct: &errorStatus,
	}

	input.vk.processExecution(input.ctx, resp)
	events := input.ctx.EventManager().Events()

	require.EqualValues(t, types.EventTypeContractStatus, events[0].Type)

	require.EqualValues(t, types.AttrKeyStatus, events[0].Attributes[0].Key)
	require.EqualValues(t, types.StatusKeep, events[0].Attributes[0].Value)

	for _, attr := range events[0].Attributes {
		require.NotEqual(t, []byte(types.StatusError), attr.Key)
	}
}

// When status still keep but returns error (and it could exists).
func TestExecKeepAndError(t *testing.T) {
	input := setupTestInput(true)
	defer closeInput(input)

	errorStatus := vm_grpc.VMStatus{
		MajorStatus: 16,
		SubStatus:   0,
		Message:     "aborted error!11111!1!!!",
	}

	resp := &vm_grpc.VMExecuteResponse{
		WriteSet:     nil,
		Events:       nil,
		Status:       vm_grpc.ContractStatus_Keep,
		StatusStruct: &errorStatus,
	}

	input.vk.processExecution(input.ctx, resp)
	events := input.ctx.EventManager().Events()

	require.EqualValues(t, types.EventTypeContractStatus, events[0].Type)
	require.EqualValues(t, types.AttrKeyStatus, events[0].Attributes[0].Key)
	require.EqualValues(t, types.StatusKeep, events[0].Attributes[0].Value)

	require.EqualValues(t, types.EventTypeContractStatus, events[1].Type)
	require.EqualValues(t, types.AttrKeyStatus, events[1].Attributes[0].Key)
	require.EqualValues(t, types.StatusError, events[1].Attributes[0].Value)

	require.EqualValues(t, types.AttrKeyMajorStatus, events[1].Attributes[1].Key)
	require.EqualValues(t, types.AttrKeySubStatus, events[1].Attributes[2].Key)
	require.EqualValues(t, types.AttrKeyMessage, events[1].Attributes[3].Key)

	require.EqualValues(t, []byte(strconv.FormatUint(errorStatus.MajorStatus, 10)), events[1].Attributes[1].Value)
	require.EqualValues(t, []byte(strconv.FormatUint(errorStatus.SubStatus, 10)), events[1].Attributes[2].Value)
	require.EqualValues(t, []byte(errorStatus.Message), events[1].Attributes[3].Value)
}

// test access path generation for oracles.
func Test_KeeperGetOracleAccessPath(t *testing.T) {
	input := setupTestInput(true)
	defer closeInput(input)

	assetCode := "eth_usdt"
	path := input.vk.GetOracleAccessPath(assetCode)
	require.Equal(t, make([]byte, common_vm.VMAddressLength), path.Address)
	require.Equal(t, "ffe300b84cc0315d7a963b504ca77202c8c38cd28bad5bce7bbe0301c806666200", hex.EncodeToString(path.Path))
}
