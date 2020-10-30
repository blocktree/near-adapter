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
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/blocktree/openwallet/v2/openwallet"
	"regexp"
)


type AddressDecoderV2 struct {
	openwallet.AddressDecoderV2Base
}

//NewAddressDecoder 地址解析器
func NewAddressDecoderV2(wm *WalletManager) *AddressDecoderV2 {
	decoder := AddressDecoderV2{}
	return &decoder
}

//AddressDecode 地址解析
func (dec *AddressDecoderV2) AddressDecode(addr string, opts ...interface{}) ([]byte, error) {
	if len(addr) != 64 {
		return nil, errors.New("cannot decode account id")
	}
	pub, err := hex.DecodeString(addr)

	if err != nil || len(pub) != 32 {
		return nil, err
	}
	return pub, nil
}

//AddressEncode 地址编码
func (dec *AddressDecoderV2) AddressEncode(hash []byte, opts ...interface{}) (string, error) {

	if len(hash) != 32 {
		return "", errors.New("invalid hash input")
	}

	return hex.EncodeToString(hash), nil
}

// AddressVerify 地址校验
func (dec *AddressDecoderV2) AddressVerify(address string, opts ...interface{}) bool {
	pattern := `^(([a-z\d]+[\-_])*[a-z\d]+\.)*([a-z\d]+[\-_])*[a-z\d]+$`

	if len(address) < 2 || len(address) > 64 {
		return false
	}

	if len(address) == 64 {
		pub, err := hex.DecodeString(address)

		if err != nil || len(pub) != 32 {
			return false
		}
		return true
	}

	matched, err := regexp.MatchString(pattern, address)
	if err != nil && matched == true {
		return true
	}

	return false
}

//PrivateKeyToWIF 私钥转WIF
func (dec *AddressDecoderV2) PrivateKeyToWIF(priv []byte, isTestnet bool) (string, error) {
	return "", fmt.Errorf("PrivateKeyToWIF not implement")
}

//PublicKeyToAddress 公钥转地址
func (dec *AddressDecoderV2) PublicKeyToAddress(pub []byte, isTestnet bool) (string, error) {

	if len(pub) != 32 {
		return "", errors.New("invalid pub input")
	}

	return hex.EncodeToString(pub), nil
}

//WIFToPrivateKey WIF转私钥
func (dec *AddressDecoderV2) WIFToPrivateKey(wif string, isTestnet bool) ([]byte, error) {
	return nil, fmt.Errorf("WIFToPrivateKey not implement")
}

//RedeemScriptToAddress 多重签名赎回脚本转地址
func (dec *AddressDecoderV2) RedeemScriptToAddress(pubs [][]byte, required uint64, isTestnet bool) (string, error) {
	return "", fmt.Errorf("RedeemScriptToAddress not implement")
}

// CustomCreateAddress 创建账户地址
func (dec *AddressDecoderV2) CustomCreateAddress(account *openwallet.AssetsAccount, newIndex uint64) (*openwallet.Address, error) {
	return nil, fmt.Errorf("CreateAddressByAccount not implement")
}

// SupportCustomCreateAddressFunction 支持创建地址实现
func (dec *AddressDecoderV2) SupportCustomCreateAddressFunction() bool {
	return false
}
