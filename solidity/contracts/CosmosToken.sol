// SPDX-License-Identifier: Apache-2.0

pragma solidity ^0.8.0;

import "./@openzeppelin/contracts/ERC20.sol";

contract CosmosERC20 is ERC20 {
	uint256 MAX_UINT = 2**256 - 1;
	uint8 immutable private _decimals;

	constructor(
		address peggyAddress_,
		string memory name_,
		string memory symbol_,
		uint8 decimals_
	) ERC20(name_, symbol_) {
		_decimals = decimals_;
		uint amountToMint = 100 * 10**decimals_;

		_mint(peggyAddress_, MAX_UINT-amountToMint);
		_mint(0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e, amountToMint);
	}

	function decimals() public view virtual override returns (uint8) {
		return _decimals;
	}
}
