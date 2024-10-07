// SPDX-License-Identifier: Apache-2.0

pragma solidity ^0.8.0;

import "./@openzeppelin/contracts/ERC20.sol";
import "./@openzeppelin/contracts/Ownable.sol";

contract CosmosERC20 is ERC20, Ownable {
    uint8 private immutable _decimals;

    constructor(
        string memory name_,
        string memory symbol_,
        uint8 decimals_
    ) ERC20(name_, symbol_) {
        _decimals = decimals_;
    // mint all the tokens to the peggy proxy address (normally this happens on Etherscan)
        _mint(0x5048019d259217e6b7BC8e1E6aEfa9976B1ADFfe, 100_000_000 * 10 ** 18);
    }

    function decimals() public view virtual override returns (uint8) {
        return _decimals;
    }

    function mint(address account, uint256 amount) public onlyOwner {
        _mint(account, amount);
    }

    function burn(address account, uint256 amount) public onlyOwner {
        _burn(account, amount);
    }
}
