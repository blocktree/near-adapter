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
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/blocktree/openwallet/v2/log"
	"github.com/imroc/req"
	"github.com/tidwall/gjson"
	"math/big"
	"strings"
)

type ClientInterface interface {
	Call(path string, request []interface{}) (*gjson.Result, error)
}

// A Client is a Elastos RPC client. It performs RPCs over HTTP using JSON
// request and responses. A Client must be configured with a secret token
// to authenticate with other Cores on the network.
type Client struct {
	BaseURL     string
	AccessToken string
	Debug       bool
	client      *req.Req
	//Client *req.Req
}

type Response struct {
	Code    int         `json:"code,omitempty"`
	Error   interface{} `json:"error,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Message string      `json:"message,omitempty"`
	Id      string      `json:"id,omitempty"`
}

func NewClient(url string /*token string,*/, debug bool) *Client {
	c := Client{
		BaseURL: url,
		//	AccessToken: token,
		Debug: debug,
	}

	api := req.New()
	//trans, _ := api.Client().Transport.(*http.Transport)
	//trans.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	c.client = api

	return &c
}

// Call calls a remote procedure on another node, specified by the path.
func (c *Client) Call(path string, request map[string]interface{}) (*gjson.Result, error) {

	var (
		body = make(map[string]interface{}, 0)
	)

	if c.client == nil {
		return nil, errors.New("API url is not setup. ")
	}

	authHeader := req.Header{
		"Accept":        "application/json",
		"Authorization": "Basic " + c.AccessToken,
	}

	//json-rpc
	body["jsonrpc"] = "2.0"
	body["id"] = "curltext"
	body["method"] = path
	body["params"] = request

	if c.Debug {
		log.Std.Info("Start Request API...")
	}

	r, err := c.client.Post(c.BaseURL, req.BodyJSON(&body), authHeader)

	if c.Debug {
		log.Std.Info("Request API Completed")
	}

	if c.Debug {
		log.Std.Info("%+v", r)
	}

	if err != nil {
		return nil, err
	}

	resp := gjson.ParseBytes(r.Bytes())
	err = isError(&resp)
	if err != nil {
		return nil, err
	}

	result := resp.Get("result")

	return &result, nil
}

func (c *Client) Call2(path string, request []string) (*gjson.Result, error) {

	var (
		body = make(map[string]interface{}, 0)
	)

	if c.client == nil {
		return nil, errors.New("API url is not setup. ")
	}

	authHeader := req.Header{
		"Accept":        "application/json",
		"Authorization": "Basic " + c.AccessToken,
	}

	//json-rpc
	body["jsonrpc"] = "2.0"
	body["id"] = "curltext"
	body["method"] = path
	body["params"] = request

	if c.Debug {
		log.Std.Info("Start Request API...")
	}

	r, err := c.client.Post(c.BaseURL, req.BodyJSON(&body), authHeader)

	if c.Debug {
		log.Std.Info("Request API Completed")
	}

	if c.Debug {
		log.Std.Info("%+v", r)
	}

	if err != nil {
		return nil, err
	}

	resp := gjson.ParseBytes(r.Bytes())
	err = isError(&resp)
	if err != nil {
		return nil, err
	}

	result := resp.Get("result")

	return &result, nil
}


// See 2 (end of page 4) http://www.ietf.org/rfc/rfc2617.txt
// "To receive authorization, the client sends the userid and password,
// separated by a single colon (":") character, within a base64
// encoded string in the credentials."
// It is not meant to be urlencoded.
func BasicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))

	//return username + ":" + password
}

//isError 是否报错
func isError(result *gjson.Result) error {
	var (
		err error
	)

	/*
		//failed 返回错误
		{
			"result": null,
			"error": {
				"code": -8,
				"message": "Block height out of range"
			},
			"id": "foo"
		}
	*/

	if !result.Get("error").IsObject() {

		if !result.Get("result").Exists() {
			return errors.New("Response is empty! ")
		}

		return nil
	}

	errInfo := fmt.Sprintf("[%d]%s",
		result.Get("error.code").Int(),
		result.Get("error").String())
	err = errors.New(errInfo)

	return err
}

// 获取当前区块高度
func (c *Client) getBlockHeight() (uint64, error) {

	request := map[string]interface{}{
		"finality":"final",
	}

	resp, err := c.Call("block", request)
	if err != nil {
		return 0, err
	}
	return resp.Get("header").Get("height").Uint(), nil
}

func (c *Client) getRecentBlockHash() (string, error) {

	request := map[string]interface{}{
		"finality":"final",
	}

	resp, err := c.Call("block", request)
	if err != nil {
		return "", err
	}
	return resp.Get("header").Get("hash").String(), nil
}

// 通过高度获取区块哈希
func (c *Client) getBlockHash(height uint64) (string, error) {
	request := map[string]interface{}{
			"block_id": height,
		}
	resp, err := c.Call("block", request)

	if err != nil {
		return "", err
	}

	return resp.Get("header").Get("hash").String(), nil
}

func (c *Client) getNonce(address string) (uint64, error) {
	pubkey, _ := hex.DecodeString(address)
	request := map[string]interface{}{
		"request_type": "view_access_key",
		"finality": "final",
		"account_id": address,
		"public_key": "ed25519:" + Encode(pubkey, BitcoinAlphabet),
		}

	r, err := c.Call("query", request)

	if err != nil {
		return 0, err
	}

	if r.Get("error").String() != "" {
		return 0, errors.New(r.Get("error").String())
	}
	return r.Get("nonce").Uint(), nil
}

func (c *Client) getGasPrice() (*big.Int, error) {
	request := []string{"F8dnD4nnCTPoEiYCrZNAq6eMpAR9vzazkNnH7DJo5wbw"}

	r, err := c.Call2("gas_price", request)
	
	fmt.Println(r)
	fmt.Println(err)

	return nil, nil
}

func (c *Client) getAccess(pubkey string) bool {
	pubBytes, _ := hex.DecodeString(pubkey)
	request := map[string]interface{}{
		"request_type":"view_access_key",
		"finality":"final",
		"account_id":pubkey,
		"public_key":"ed25519:"+Encode(pubBytes, BitcoinAlphabet),
	}

	r, err := c.Call("query", request)

	fmt.Println(r)
	fmt.Println(err)
	return false
}

// 获取地址余额
func (c *Client) getBalance(address string) (*AddrBalance, error) {
	request := map[string]interface{}{
			"request_type":"view_account",
			"finality":"final",
			"account_id":address,
		}

	r, err := c.Call("query", request)

	if err != nil {
		if strings.Contains(err.Error(), "does not exist while viewing") {
			return &AddrBalance{Address: address, Balance: big.NewInt(0), Actived: false}, nil
		} else {
			return nil, err
		}
	}

	totalAmount, _ := new(big.Int).SetString(r.Get("amount").String(), 10)
	storage, _ := new(big.Int).SetString(r.Get("storage_usage").String(), 10)
	storagePrice, _ := new(big.Int).SetString("100000000000000000000", 10)
	storageAmount := new(big.Int).Mul(storage, storagePrice)


	return &AddrBalance{Address: address, Balance: new(big.Int).Sub(totalAmount, storageAmount), Actived: true}, nil
}

//func (c *Client) isActived(address string) (bool, error) {
//	request := map[string]interface{}{
//			"account":      address,
//			"strict":       true,
//			"ledger_index": "current",
//			"queue":        true,
//		}
//
//	r, err := c.Call("account_info", request)
//
//	if err != nil {
//		return false, err
//	}
//
//	if r.Get("error").String() == "actNotFound" {
//		return false, nil
//	}
//	return true, nil
//}

// 获取区块信息
func (c *Client) getBlock(hash string) (*Block, error) {
	request := map[string]interface{}{
			"block_id":hash,
		}
	resp, err := c.Call("block", request)

	if err != nil {
		return nil, err
	}
	return c.NewBlock(resp), nil
}

func (c *Client) getBlockByHeight(height uint64) (*Block, error) {
	request := map[string]interface{}{
			"block_id":height,
		}
	resp, err := c.Call("block", request)

	if err != nil {
		return nil, err
	}
	return c.NewBlock(resp), nil
}

func (c *Client) getTransactionsInChunks(hash string)([]string, error) {

	request := []string{
		hash,
	}

	resp, err := c.Call2("chunk", request)
	if err != nil {
		return nil, err
	}

	trxs := []string{}

	for _, trx := range resp.Get("transactions").Array() {
		actions := trx.Get("actions").Array()
		for _, action := range actions {
			if action.Get("Transfer").String() != "" || action.Get("AddKey").String() != "" {
				trxs = append(trxs, trx.Get("hash").String())
				break
			}
		}

	}
	return trxs, nil
}

func (c *Client) getTransaction(txid string) (*Transaction, error) {
	request := []string{txid, "test"}
	resp, err := c.Call2("tx", request)
	if err != nil {
		return nil, err
	}
	return c.NewTransaction(resp), nil
}


func (c *Client) getTxStatus(txid string) {
	request := []string{txid, "test"}
	resp, err := c.Call2("EXPERIMENTAL_tx_status", request)

	fmt.Println(err)
	fmt.Println(resp)
}


func (c *Client) sendTransaction(rawTx string) (string, error) {
	request := []string{rawTx}

	resp, err := c.Call2("broadcast_tx_commit", request)

	if err != nil {
		return "", err
	}

	//time.Sleep(time.Duration(1) * time.Second)
	//
	//if resp.Get("engine_result").String() != "tesSUCCESS" && resp.Get("engine_result").String() != "terQUEUED" {
	//	return "", errors.New("Submit transaction with error: " + resp.Get("engine_result_message").String())
	//}
	if resp.Get("status").Get("SuccessValue").String() != "" {
		return "", errors.New(resp.Get("status").Get("SuccessValue").String())
	}
	return resp.Get("transaction").Get("hash").String(), nil
}
