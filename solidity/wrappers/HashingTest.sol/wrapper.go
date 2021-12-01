// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package wrappers

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
)

// HashingTestMetaData contains all meta data concerning the HashingTest contract.
var HashingTestMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"address[]\",\"name\":\"_validators\",\"type\":\"address[]\"},{\"internalType\":\"uint256[]\",\"name\":\"_powers\",\"type\":\"uint256[]\"},{\"internalType\":\"uint256\",\"name\":\"_valsetNonce\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"_rewardAmount\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"_rewardToken\",\"type\":\"address\"},{\"internalType\":\"bytes32\",\"name\":\"_peggyId\",\"type\":\"bytes32\"}],\"name\":\"CheckpointHash\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address[]\",\"name\":\"_validators\",\"type\":\"address[]\"},{\"internalType\":\"uint256[]\",\"name\":\"_powers\",\"type\":\"uint256[]\"},{\"internalType\":\"uint256\",\"name\":\"_valsetNonce\",\"type\":\"uint256\"}],\"name\":\"JustSaveEverything\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address[]\",\"name\":\"_validators\",\"type\":\"address[]\"},{\"internalType\":\"uint256[]\",\"name\":\"_powers\",\"type\":\"uint256[]\"},{\"internalType\":\"uint256\",\"name\":\"_valsetNonce\",\"type\":\"uint256\"}],\"name\":\"JustSaveEverythingAgain\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"lastCheckpoint\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"state_nonce\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"state_powers\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"state_validators\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"}]",
	Sigs: map[string]string{
		"cd16f185": "CheckpointHash(address[],uint256[],uint256,uint256,address,bytes32)",
		"884403e2": "JustSaveEverything(address[],uint256[],uint256)",
		"715dff7e": "JustSaveEverythingAgain(address[],uint256[],uint256)",
		"d32e81a5": "lastCheckpoint()",
		"ccf0e74c": "state_nonce()",
		"2b939281": "state_powers(uint256)",
		"2afbb62e": "state_validators(uint256)",
	},
	Bin: "0x608060405234801561001057600080fd5b506105f9806100206000396000f3fe608060405234801561001057600080fd5b506004361061007d5760003560e01c8063884403e21161005b578063884403e2146100d3578063ccf0e74c146100e8578063cd16f185146100f1578063d32e81a5146101045761007d565b80632afbb62e146100825780632b939281146100b2578063715dff7e146100d3575b600080fd5b610095610090366004610470565b61010d565b6040516001600160a01b0390911681526020015b60405180910390f35b6100c56100c0366004610470565b610137565b6040519081526020016100a9565b6100e66100e1366004610379565b610158565b005b6100c560035481565b6100e66100ff3660046103e3565b610187565b6100c560005481565b6001818154811061011d57600080fd5b6000918252602090912001546001600160a01b0316905081565b6002818154811061014757600080fd5b600091825260209091200154905081565b825161016b9060019060208601906101db565b50815161017f906002906020850190610240565b506003555050565b6040516918da1958dadc1bda5b9d60b21b906000906101b6908490849089908c908c908b908b906020016104c2565b60408051601f1981840301815291905280516020909101206000555050505050505050565b828054828255906000526020600020908101928215610230579160200282015b8281111561023057825182546001600160a01b0319166001600160a01b039091161782556020909201916001909101906101fb565b5061023c92915061027b565b5090565b828054828255906000526020600020908101928215610230579160200282015b82811115610230578251825591602001919060010190610260565b5b8082111561023c576000815560010161027c565b80356001600160a01b03811681146102a757600080fd5b919050565b600082601f8301126102bc578081fd5b813560206102d16102cc83610589565b610558565b82815281810190858301838502870184018810156102ed578586fd5b855b858110156103125761030082610290565b845292840192908401906001016102ef565b5090979650505050505050565b600082601f83011261032f578081fd5b8135602061033f6102cc83610589565b828152818101908583018385028701840188101561035b578586fd5b855b858110156103125781358452928401929084019060010161035d565b60008060006060848603121561038d578283fd5b833567ffffffffffffffff808211156103a4578485fd5b6103b0878388016102ac565b945060208601359150808211156103c5578384fd5b506103d28682870161031f565b925050604084013590509250925092565b60008060008060008060c087890312156103fb578182fd5b863567ffffffffffffffff80821115610412578384fd5b61041e8a838b016102ac565b97506020890135915080821115610433578384fd5b5061044089828a0161031f565b955050604087013593506060870135925061045d60808801610290565b915060a087013590509295509295509295565b600060208284031215610481578081fd5b5035919050565b6000815180845260208085019450808401835b838110156104b75781518752958201959082019060010161049b565b509495945050505050565b600060e082018983526020898185015288604085015260e0606085015281885180845261010086019150828a019350845b818110156105185784516001600160a01b0316835293830193918301916001016104f3565b5050848103608086015261052c8189610488565b93505050508360a083015261054c60c08301846001600160a01b03169052565b98975050505050505050565b604051601f8201601f1916810167ffffffffffffffff81118282101715610581576105816105ad565b604052919050565b600067ffffffffffffffff8211156105a3576105a36105ad565b5060209081020190565b634e487b7160e01b600052604160045260246000fdfea26469706673582212208a3bb97c7fd3dbd7942e1e0d4bd37c337594ad759dc5cf7e9af17de2c4dcae9764736f6c63430008020033",
}

// HashingTestABI is the input ABI used to generate the binding from.
// Deprecated: Use HashingTestMetaData.ABI instead.
var HashingTestABI = HashingTestMetaData.ABI

// Deprecated: Use HashingTestMetaData.Sigs instead.
// HashingTestFuncSigs maps the 4-byte function signature to its string representation.
var HashingTestFuncSigs = HashingTestMetaData.Sigs

// HashingTestBin is the compiled bytecode used for deploying new contracts.
// Deprecated: Use HashingTestMetaData.Bin instead.
var HashingTestBin = HashingTestMetaData.Bin

// DeployHashingTest deploys a new Ethereum contract, binding an instance of HashingTest to it.
func DeployHashingTest(auth *bind.TransactOpts, backend bind.ContractBackend) (common.Address, *types.Transaction, *HashingTest, error) {
	parsed, err := HashingTestMetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	if parsed == nil {
		return common.Address{}, nil, nil, errors.New("GetABI returned nil")
	}

	address, tx, contract, err := bind.DeployContract(auth, *parsed, common.FromHex(HashingTestBin), backend)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &HashingTest{HashingTestCaller: HashingTestCaller{contract: contract}, HashingTestTransactor: HashingTestTransactor{contract: contract}, HashingTestFilterer: HashingTestFilterer{contract: contract}}, nil
}

// HashingTest is an auto generated Go binding around an Ethereum contract.
type HashingTest struct {
	HashingTestCaller     // Read-only binding to the contract
	HashingTestTransactor // Write-only binding to the contract
	HashingTestFilterer   // Log filterer for contract events
}

// HashingTestCaller is an auto generated read-only Go binding around an Ethereum contract.
type HashingTestCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// HashingTestTransactor is an auto generated write-only Go binding around an Ethereum contract.
type HashingTestTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// HashingTestFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type HashingTestFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// HashingTestSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type HashingTestSession struct {
	Contract     *HashingTest      // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// HashingTestCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type HashingTestCallerSession struct {
	Contract *HashingTestCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts      // Call options to use throughout this session
}

// HashingTestTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type HashingTestTransactorSession struct {
	Contract     *HashingTestTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts      // Transaction auth options to use throughout this session
}

// HashingTestRaw is an auto generated low-level Go binding around an Ethereum contract.
type HashingTestRaw struct {
	Contract *HashingTest // Generic contract binding to access the raw methods on
}

// HashingTestCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type HashingTestCallerRaw struct {
	Contract *HashingTestCaller // Generic read-only contract binding to access the raw methods on
}

// HashingTestTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type HashingTestTransactorRaw struct {
	Contract *HashingTestTransactor // Generic write-only contract binding to access the raw methods on
}

// NewHashingTest creates a new instance of HashingTest, bound to a specific deployed contract.
func NewHashingTest(address common.Address, backend bind.ContractBackend) (*HashingTest, error) {
	contract, err := bindHashingTest(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &HashingTest{HashingTestCaller: HashingTestCaller{contract: contract}, HashingTestTransactor: HashingTestTransactor{contract: contract}, HashingTestFilterer: HashingTestFilterer{contract: contract}}, nil
}

// NewHashingTestCaller creates a new read-only instance of HashingTest, bound to a specific deployed contract.
func NewHashingTestCaller(address common.Address, caller bind.ContractCaller) (*HashingTestCaller, error) {
	contract, err := bindHashingTest(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &HashingTestCaller{contract: contract}, nil
}

// NewHashingTestTransactor creates a new write-only instance of HashingTest, bound to a specific deployed contract.
func NewHashingTestTransactor(address common.Address, transactor bind.ContractTransactor) (*HashingTestTransactor, error) {
	contract, err := bindHashingTest(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &HashingTestTransactor{contract: contract}, nil
}

// NewHashingTestFilterer creates a new log filterer instance of HashingTest, bound to a specific deployed contract.
func NewHashingTestFilterer(address common.Address, filterer bind.ContractFilterer) (*HashingTestFilterer, error) {
	contract, err := bindHashingTest(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &HashingTestFilterer{contract: contract}, nil
}

// bindHashingTest binds a generic wrapper to an already deployed contract.
func bindHashingTest(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(HashingTestABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_HashingTest *HashingTestRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _HashingTest.Contract.HashingTestCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_HashingTest *HashingTestRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _HashingTest.Contract.HashingTestTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_HashingTest *HashingTestRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _HashingTest.Contract.HashingTestTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_HashingTest *HashingTestCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _HashingTest.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_HashingTest *HashingTestTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _HashingTest.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_HashingTest *HashingTestTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _HashingTest.Contract.contract.Transact(opts, method, params...)
}

// LastCheckpoint is a free data retrieval call binding the contract method 0xd32e81a5.
//
// Solidity: function lastCheckpoint() view returns(bytes32)
func (_HashingTest *HashingTestCaller) LastCheckpoint(opts *bind.CallOpts) ([32]byte, error) {
	var out []interface{}
	err := _HashingTest.contract.Call(opts, &out, "lastCheckpoint")

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// LastCheckpoint is a free data retrieval call binding the contract method 0xd32e81a5.
//
// Solidity: function lastCheckpoint() view returns(bytes32)
func (_HashingTest *HashingTestSession) LastCheckpoint() ([32]byte, error) {
	return _HashingTest.Contract.LastCheckpoint(&_HashingTest.CallOpts)
}

// LastCheckpoint is a free data retrieval call binding the contract method 0xd32e81a5.
//
// Solidity: function lastCheckpoint() view returns(bytes32)
func (_HashingTest *HashingTestCallerSession) LastCheckpoint() ([32]byte, error) {
	return _HashingTest.Contract.LastCheckpoint(&_HashingTest.CallOpts)
}

// StateNonce is a free data retrieval call binding the contract method 0xccf0e74c.
//
// Solidity: function state_nonce() view returns(uint256)
func (_HashingTest *HashingTestCaller) StateNonce(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _HashingTest.contract.Call(opts, &out, "state_nonce")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// StateNonce is a free data retrieval call binding the contract method 0xccf0e74c.
//
// Solidity: function state_nonce() view returns(uint256)
func (_HashingTest *HashingTestSession) StateNonce() (*big.Int, error) {
	return _HashingTest.Contract.StateNonce(&_HashingTest.CallOpts)
}

// StateNonce is a free data retrieval call binding the contract method 0xccf0e74c.
//
// Solidity: function state_nonce() view returns(uint256)
func (_HashingTest *HashingTestCallerSession) StateNonce() (*big.Int, error) {
	return _HashingTest.Contract.StateNonce(&_HashingTest.CallOpts)
}

// StatePowers is a free data retrieval call binding the contract method 0x2b939281.
//
// Solidity: function state_powers(uint256 ) view returns(uint256)
func (_HashingTest *HashingTestCaller) StatePowers(opts *bind.CallOpts, arg0 *big.Int) (*big.Int, error) {
	var out []interface{}
	err := _HashingTest.contract.Call(opts, &out, "state_powers", arg0)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// StatePowers is a free data retrieval call binding the contract method 0x2b939281.
//
// Solidity: function state_powers(uint256 ) view returns(uint256)
func (_HashingTest *HashingTestSession) StatePowers(arg0 *big.Int) (*big.Int, error) {
	return _HashingTest.Contract.StatePowers(&_HashingTest.CallOpts, arg0)
}

// StatePowers is a free data retrieval call binding the contract method 0x2b939281.
//
// Solidity: function state_powers(uint256 ) view returns(uint256)
func (_HashingTest *HashingTestCallerSession) StatePowers(arg0 *big.Int) (*big.Int, error) {
	return _HashingTest.Contract.StatePowers(&_HashingTest.CallOpts, arg0)
}

// StateValidators is a free data retrieval call binding the contract method 0x2afbb62e.
//
// Solidity: function state_validators(uint256 ) view returns(address)
func (_HashingTest *HashingTestCaller) StateValidators(opts *bind.CallOpts, arg0 *big.Int) (common.Address, error) {
	var out []interface{}
	err := _HashingTest.contract.Call(opts, &out, "state_validators", arg0)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// StateValidators is a free data retrieval call binding the contract method 0x2afbb62e.
//
// Solidity: function state_validators(uint256 ) view returns(address)
func (_HashingTest *HashingTestSession) StateValidators(arg0 *big.Int) (common.Address, error) {
	return _HashingTest.Contract.StateValidators(&_HashingTest.CallOpts, arg0)
}

// StateValidators is a free data retrieval call binding the contract method 0x2afbb62e.
//
// Solidity: function state_validators(uint256 ) view returns(address)
func (_HashingTest *HashingTestCallerSession) StateValidators(arg0 *big.Int) (common.Address, error) {
	return _HashingTest.Contract.StateValidators(&_HashingTest.CallOpts, arg0)
}

// CheckpointHash is a paid mutator transaction binding the contract method 0xcd16f185.
//
// Solidity: function CheckpointHash(address[] _validators, uint256[] _powers, uint256 _valsetNonce, uint256 _rewardAmount, address _rewardToken, bytes32 _peggyId) returns()
func (_HashingTest *HashingTestTransactor) CheckpointHash(opts *bind.TransactOpts, _validators []common.Address, _powers []*big.Int, _valsetNonce *big.Int, _rewardAmount *big.Int, _rewardToken common.Address, _peggyId [32]byte) (*types.Transaction, error) {
	return _HashingTest.contract.Transact(opts, "CheckpointHash", _validators, _powers, _valsetNonce, _rewardAmount, _rewardToken, _peggyId)
}

// CheckpointHash is a paid mutator transaction binding the contract method 0xcd16f185.
//
// Solidity: function CheckpointHash(address[] _validators, uint256[] _powers, uint256 _valsetNonce, uint256 _rewardAmount, address _rewardToken, bytes32 _peggyId) returns()
func (_HashingTest *HashingTestSession) CheckpointHash(_validators []common.Address, _powers []*big.Int, _valsetNonce *big.Int, _rewardAmount *big.Int, _rewardToken common.Address, _peggyId [32]byte) (*types.Transaction, error) {
	return _HashingTest.Contract.CheckpointHash(&_HashingTest.TransactOpts, _validators, _powers, _valsetNonce, _rewardAmount, _rewardToken, _peggyId)
}

// CheckpointHash is a paid mutator transaction binding the contract method 0xcd16f185.
//
// Solidity: function CheckpointHash(address[] _validators, uint256[] _powers, uint256 _valsetNonce, uint256 _rewardAmount, address _rewardToken, bytes32 _peggyId) returns()
func (_HashingTest *HashingTestTransactorSession) CheckpointHash(_validators []common.Address, _powers []*big.Int, _valsetNonce *big.Int, _rewardAmount *big.Int, _rewardToken common.Address, _peggyId [32]byte) (*types.Transaction, error) {
	return _HashingTest.Contract.CheckpointHash(&_HashingTest.TransactOpts, _validators, _powers, _valsetNonce, _rewardAmount, _rewardToken, _peggyId)
}

// JustSaveEverything is a paid mutator transaction binding the contract method 0x884403e2.
//
// Solidity: function JustSaveEverything(address[] _validators, uint256[] _powers, uint256 _valsetNonce) returns()
func (_HashingTest *HashingTestTransactor) JustSaveEverything(opts *bind.TransactOpts, _validators []common.Address, _powers []*big.Int, _valsetNonce *big.Int) (*types.Transaction, error) {
	return _HashingTest.contract.Transact(opts, "JustSaveEverything", _validators, _powers, _valsetNonce)
}

// JustSaveEverything is a paid mutator transaction binding the contract method 0x884403e2.
//
// Solidity: function JustSaveEverything(address[] _validators, uint256[] _powers, uint256 _valsetNonce) returns()
func (_HashingTest *HashingTestSession) JustSaveEverything(_validators []common.Address, _powers []*big.Int, _valsetNonce *big.Int) (*types.Transaction, error) {
	return _HashingTest.Contract.JustSaveEverything(&_HashingTest.TransactOpts, _validators, _powers, _valsetNonce)
}

// JustSaveEverything is a paid mutator transaction binding the contract method 0x884403e2.
//
// Solidity: function JustSaveEverything(address[] _validators, uint256[] _powers, uint256 _valsetNonce) returns()
func (_HashingTest *HashingTestTransactorSession) JustSaveEverything(_validators []common.Address, _powers []*big.Int, _valsetNonce *big.Int) (*types.Transaction, error) {
	return _HashingTest.Contract.JustSaveEverything(&_HashingTest.TransactOpts, _validators, _powers, _valsetNonce)
}

// JustSaveEverythingAgain is a paid mutator transaction binding the contract method 0x715dff7e.
//
// Solidity: function JustSaveEverythingAgain(address[] _validators, uint256[] _powers, uint256 _valsetNonce) returns()
func (_HashingTest *HashingTestTransactor) JustSaveEverythingAgain(opts *bind.TransactOpts, _validators []common.Address, _powers []*big.Int, _valsetNonce *big.Int) (*types.Transaction, error) {
	return _HashingTest.contract.Transact(opts, "JustSaveEverythingAgain", _validators, _powers, _valsetNonce)
}

// JustSaveEverythingAgain is a paid mutator transaction binding the contract method 0x715dff7e.
//
// Solidity: function JustSaveEverythingAgain(address[] _validators, uint256[] _powers, uint256 _valsetNonce) returns()
func (_HashingTest *HashingTestSession) JustSaveEverythingAgain(_validators []common.Address, _powers []*big.Int, _valsetNonce *big.Int) (*types.Transaction, error) {
	return _HashingTest.Contract.JustSaveEverythingAgain(&_HashingTest.TransactOpts, _validators, _powers, _valsetNonce)
}

// JustSaveEverythingAgain is a paid mutator transaction binding the contract method 0x715dff7e.
//
// Solidity: function JustSaveEverythingAgain(address[] _validators, uint256[] _powers, uint256 _valsetNonce) returns()
func (_HashingTest *HashingTestTransactorSession) JustSaveEverythingAgain(_validators []common.Address, _powers []*big.Int, _valsetNonce *big.Int) (*types.Transaction, error) {
	return _HashingTest.Contract.JustSaveEverythingAgain(&_HashingTest.TransactOpts, _validators, _powers, _valsetNonce)
}
