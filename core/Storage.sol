// SPDX-License-Identifier: GPL-3.0

pragma solidity >=0.7.0 <=0.8.9;

/**
 * @title Storage
 * @dev Store & retrieve value in a variable
 */
contract Storage {

    uint256 number;

    /**
     * @dev Store value in variable
     * @param num value to store
     */
    function store(uint256 num) public {
        number = num;
    }

    /**
     * @dev Return value 
     * @return value of 'number'
     */
    function retrieve() public view returns (uint256){
        return number;
    }
}

contract StorageManager {
    // track storage contracts we've created
    
    address[] public storageAddrs;
    
    function create() public {
        Storage strContract = new Storage();
        storageAddrs.push(address(strContract));
    }
    
    function store(uint256 index, uint256 num) public {
        require(index < storageAddrs.length, "invalid index");
        Storage strContract = Storage(storageAddrs[index]);
        strContract.store(num);
    }
    
    function retrieve(uint256 index) public view returns (uint256) {
        require(index < storageAddrs.length, "invalid index");
        Storage strContract = Storage(storageAddrs[index]);
        return strContract.retrieve();
    }
}
