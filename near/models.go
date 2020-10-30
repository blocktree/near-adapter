/*
 * Copyright 2018 The openwallet Authors
 * This file is part of the openwallet library.
 *
 * The openwallet library is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * The openwallet library is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU Lesser General Public License for more details.
 */

package near

import (
	"fmt"
	"github.com/blocktree/openwallet/v2/crypto"
	"github.com/blocktree/openwallet/v2/openwallet"
	"github.com/ethereum/go-ethereum/common"
	"github.com/tidwall/gjson"
	"math/big"
)

// type Vin struct {
// 	Coinbase string
// 	TxID     string
// 	Vout     uint64
// 	N        uint64
// 	Addr     string
// 	Value    string
// }

// type Vout struct {
// 	N            uint64
// 	Addr         string
// 	Value        string
// 	ScriptPubKey string
// 	Type         string
// }

type Block struct {
	Hash                  string // actually block signature in M chain
	PrevBlockHash         string // actually block signature in M chain
	TransactionMerkleRoot string
	Timestamp             uint64
	Height                uint64
	Transactions          []string
}

type Transaction struct {
	TxType         string
	TxID           string
	Fee            *big.Int
	From           string
	To             string
	TimeStamp      uint64
	Amount         *big.Int
	BlockHeight    uint64
	BlockHash      string
	Status         string
}

func (c *Client) NewTransaction(json *gjson.Result) *Transaction {

	obj := &Transaction{}
	actions := gjson.Get(json.Raw, "transaction").Get("actions").Array()

	for _, action := range actions {
		if action.Get("Transfer").String() != "" {
			obj.TxType = "transfer"
			break
		}
		if action.Get("AddKey").String() != "" {
			obj.TxType = "addKey"
		}
	}

	if obj.TxType == "" {
		return obj
	}

	obj.TxID = gjson.Get(json.Raw, "transaction").Get("hash").String()
	fee := new(big.Int)
	if gjson.Get(json.Raw, "transaction_outcome").Get("outcome").Get("tokens_burnt").String() != "" {
		outcomeFee, _ := new(big.Int).SetString(gjson.Get(json.Raw, "transaction_outcome").Get("outcome").Get("tokens_burnt").String(), 10)
		fee.Add(fee, outcomeFee)
	}

	receipts_outcome := gjson.Get(json.Raw, "receipts_outcome").Array()

	for _, receipt := range receipts_outcome {
		if receipt.Get("outcome").Get("tokens_burnt").String() != "" {
			outcomeFee, _ := new(big.Int).SetString(receipt.Get("outcome").Get("tokens_burnt").String(), 10)
			fee.Add(fee, outcomeFee)
		}
	}
	obj.Fee = fee

	obj.From = gjson.Get(json.Raw, "transaction").Get("signer_id").String()
	obj.BlockHash = gjson.Get(json.Raw, "transaction_outcome").Get("block_hash").String()
	block, err := c.getBlock(obj.BlockHash)
	if err != nil {
		return nil
	}
	obj.BlockHeight = block.Height
	obj.TimeStamp = block.Timestamp
	amount := new(big.Int)
	for _, action := range actions {
		if action.Get("Transfer").String() != "" {
			if action.Get("Transfer").Get("deposit").String() != "" {
				deposit, _ := new(big.Int).SetString(action.Get("Transfer").Get("deposit").String(), 10)
				amount.Add(amount, deposit)
			}
		}
	}

	obj.Amount = amount
	obj.To = gjson.Get(json.Raw, "transaction").Get("receiver_id").String()
	obj.Status = gjson.Get(json.Raw, "status").Get("SuccessValue").String()

	return obj
}


func (c *Client)NewBlock(json *gjson.Result) *Block {
	obj := &Block{}
	// 解  析
	obj.Hash = gjson.Get(json.Raw, "header").Get("hash").String()
	obj.PrevBlockHash = gjson.Get(json.Raw, "header").Get("prev_hash").String()
	obj.TransactionMerkleRoot = gjson.Get(json.Raw, "header").Get("chunk_tx_root").String()
	obj.Timestamp = gjson.Get(json.Raw, "header").Get("timestamp").Uint()
	obj.Height = gjson.Get(json.Raw, "header").Get("height").Uint()

	chunks := gjson.Get(json.Raw, "chunks").Array()
	for _, chunk := range chunks {
		trxs, _ := c.getTransactionsInChunks(chunk.Get("chunk_hash").String())
		obj.Transactions = append(obj.Transactions, trxs...)
	}

	return obj
}

//BlockHeader 区块链头
func (b *Block) BlockHeader() *openwallet.BlockHeader {

	obj := openwallet.BlockHeader{}
	//解析json
	obj.Hash = b.Hash
	//obj.Confirmations = b.Confirmations
	obj.Merkleroot = b.TransactionMerkleRoot
	obj.Previousblockhash = b.PrevBlockHash
	obj.Height = b.Height
	//obj.Version = uint64(b.Version)
	obj.Time = b.Timestamp
	obj.Symbol = Symbol

	return &obj
}

//UnscanRecords 扫描失败的区块及交易
type UnscanRecord struct {
	ID          string `storm:"id"` // primary key
	BlockHeight uint64
	TxID        string
	Reason      string
}

func NewUnscanRecord(height uint64, txID, reason string) *UnscanRecord {
	obj := UnscanRecord{}
	obj.BlockHeight = height
	obj.TxID = txID
	obj.Reason = reason
	obj.ID = common.Bytes2Hex(crypto.SHA256([]byte(fmt.Sprintf("%d_%s", height, txID))))
	return &obj
}
