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

package openwtester

import (
	"testing"

	"github.com/blocktree/openwallet/v2/log"
	"github.com/blocktree/openwallet/v2/openw"
	"github.com/blocktree/openwallet/v2/openwallet"
)

func testGetAssetsAccountBalance(tm *openw.WalletManager, walletID, accountID string) {
	balance, err := tm.GetAssetsAccountBalance(testApp, walletID, accountID)
	if err != nil {
		log.Error("GetAssetsAccountBalance failed, unexpected error:", err)
		return
	}
	log.Info("balance:", balance)
}

func testGetAssetsAccountTokenBalance(tm *openw.WalletManager, walletID, accountID string, contract openwallet.SmartContract) {
	balance, err := tm.GetAssetsAccountTokenBalance(testApp, walletID, accountID, contract)
	if err != nil {
		log.Error("GetAssetsAccountTokenBalance failed, unexpected error:", err)
		return
	}
	log.Info("token balance:", balance.Balance)
}

func testCreateTransactionStep(tm *openw.WalletManager, walletID, accountID, to, amount, feeRate string, contract *openwallet.SmartContract) (*openwallet.RawTransaction, error) {

	//err := tm.RefreshAssetsAccountBalance(testApp, accountID)
	//if err != nil {
	//	log.Error("RefreshAssetsAccountBalance failed, unexpected error:", err)
	//	return nil, err
	//}

	rawTx, err := tm.CreateTransaction(testApp, walletID, accountID, amount, to, feeRate, "test", contract)

	if err != nil {
		log.Error("CreateTransaction failed, unexpected error:", err)
		return nil, err
	}

	return rawTx, nil
}

func testCreateSummaryTransactionStep(
	tm *openw.WalletManager,
	walletID, accountID, summaryAddress, minTransfer, retainedBalance, feeRate string,
	start, limit int,
	contract *openwallet.SmartContract) ([]*openwallet.RawTransactionWithError, error) {

	rawTxArray, err := tm.CreateSummaryRawTransactionWithError(testApp, walletID, accountID, summaryAddress, minTransfer,
		retainedBalance, feeRate, start, limit, contract, nil)

	if err != nil {
		log.Error("CreateSummaryTransaction failed, unexpected error:", err)
		return nil, err
	}

	return rawTxArray, nil
}

func testSignTransactionStep(tm *openw.WalletManager, rawTx *openwallet.RawTransaction) (*openwallet.RawTransaction, error) {

	_, err := tm.SignTransaction(testApp, rawTx.Account.WalletID, rawTx.Account.AccountID, "12345678", rawTx)
	if err != nil {
		log.Error("SignTransaction failed, unexpected error:", err)
		return nil, err
	}

	log.Infof("rawTx: %+v", rawTx)
	return rawTx, nil
}

func testVerifyTransactionStep(tm *openw.WalletManager, rawTx *openwallet.RawTransaction) (*openwallet.RawTransaction, error) {

	//log.Info("rawTx.Signatures:", rawTx.Signatures)

	_, err := tm.VerifyTransaction(testApp, rawTx.Account.WalletID, rawTx.Account.AccountID, rawTx)
	if err != nil {
		log.Error("VerifyTransaction failed, unexpected error:", err)
		return nil, err
	}

	log.Infof("rawTx: %+v", rawTx)
	return rawTx, nil
}

func testSubmitTransactionStep(tm *openw.WalletManager, rawTx *openwallet.RawTransaction) (*openwallet.RawTransaction, error) {

	tx, err := tm.SubmitTransaction(testApp, rawTx.Account.WalletID, rawTx.Account.AccountID, rawTx)
	if err != nil {
		log.Error("SubmitTransaction failed, unexpected error:", err)
		return nil, err
	}

	log.Std.Info("tx: %+v", tx)
	log.Info("wxID:", tx.WxID)
	log.Info("txID:", rawTx.TxID)

	return rawTx, nil
}

/*

wallet : WJX9kac46kUuaWS6cHhknUmYg33QgAXsQ9
account : 2xh2GfXd674Er2Xi77Kgry2XPC4cP4PESZQ73JWz8V9X
2 address : rP9YxN6yjw5HJj5LeK55gtVr8RznEPLwRc
			rnsa2e5jHVmUKbTBEmysZP2KLK54ekBkkR
---------------
wallet : WKjQEx8W6dmEx4nmzoE1utQoZeq4HC5fSZ
account : 9KKHEYZMFQJivQKkwCRTzPuQxUK2Arp71HRVP6zxK7Sj
1 address : rPASzbJFtmfExwtP4yb52GhckPKWHuAyiT
*/

func TestTransfer(t *testing.T) {
	tm := testInitWalletManager()
	walletID := "WKfekxS7RSAU1pqb6u79HpWoYwkJvm7emx"
	accountID := "Gryk5QZjnkiZsYqGneL9bJFzPC1Cw5NBfkBzeVKmfff9"
	to := "71a2caf1b6f369d64dc2ca2db950d7b296bfbe958a416fa294a83dbcd2cbe6f1"

	testGetAssetsAccountBalance(tm, walletID, accountID)

	rawTx, err := testCreateTransactionStep(tm, walletID, accountID, to, "4.88", "", nil)
	if err != nil {
		return
	}

	log.Std.Info("rawTx: %+v", rawTx)

	_, err = testSignTransactionStep(tm, rawTx)
	if err != nil {
		return
	}

	_, err = testVerifyTransactionStep(tm, rawTx)
	if err != nil {
		return
	}

	_, err = testSubmitTransactionStep(tm, rawTx)
	if err != nil {
		return
	}

	//to = "rnsa2e5jHVmUKbTBEmysZP2KLK54ekBkkR"
	//rawTx, err = testCreateTransactionStep(tm, walletID, accountID, to, "1.000012", "", nil)
	//if err != nil {
	//	return
	//}
	//
	//log.Std.Info("rawTx: %+v", rawTx)
	//
	//_, err = testSignTransactionStep(tm, rawTx)
	//if err != nil {
	//	return
	//}
	//
	//_, err = testVerifyTransactionStep(tm, rawTx)
	//if err != nil {
	//	return
	//}
	//
	//_, err = testSubmitTransactionStep(tm, rawTx)
	//if err != nil {
	//	return
	//}

}

func TestSummary(t *testing.T) {
	tm := testInitWalletManager()
	walletID := "WJX9kac46kUuaWS6cHhknUmYg33QgAXsQ9"
	accountID := "2xh2GfXd674Er2Xi77Kgry2XPC4cP4PESZQ73JWz8V9X"
	summaryAddress := "rPASzbJFtmfExwtP4yb52GhckPKWHuAyiT"

	testGetAssetsAccountBalance(tm, walletID, accountID)

	rawTxArray, err := testCreateSummaryTransactionStep(tm, walletID, accountID,
		summaryAddress, "", "", "",
		0, 100, nil)
	if err != nil {
		log.Errorf("CreateSummaryTransaction failed, unexpected error: %v", err)
		return
	}

	//执行汇总交易
	for _, rawTxWithErr := range rawTxArray {

		if rawTxWithErr.Error != nil {
			log.Error(rawTxWithErr.Error.Error())
			continue
		}

		_, err = testSignTransactionStep(tm, rawTxWithErr.RawTx)
		if err != nil {
			return
		}

		_, err = testVerifyTransactionStep(tm, rawTxWithErr.RawTx)
		if err != nil {
			return
		}

		_, err = testSubmitTransactionStep(tm, rawTxWithErr.RawTx)
		if err != nil {
			return
		}
	}

}
