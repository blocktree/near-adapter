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
	"errors"
	"fmt"
	"math/big"
	"net/url"

	"github.com/asdine/storm"
	"github.com/blocktree/openwallet/v2/common"
	"github.com/blocktree/openwallet/v2/openwallet"
	"github.com/graarh/golang-socketio"
	"github.com/graarh/golang-socketio/transport"
	"github.com/shopspring/decimal"
	"strings"
	"time"
	//"github.com/blocktree/go-owcdrivers/rippleTransaction"
)

const (
	//blockchainBucket  = "blockchain" //区块链数据集合
	maxExtractingSize = 20 //并发的扫描线程数
)

//NBlockScanner ontology的区块链扫描器
type NBlockScanner struct {
	*openwallet.BlockScannerBase

	CurrentBlockHeight   uint64             //当前区块高度
	extractingCH         chan struct{}      //扫描工作令牌
	wm                   *WalletManager     //钱包管理者
	IsScanMemPool        bool               //是否扫描交易池
	RescanLastBlockCount uint64             //重扫上N个区块数量
	socketIO             *gosocketio.Client //socketIO客户端
	RPCServer            int
}

//ExtractResult 扫描完成的提取结果
type ExtractResult struct {
	extractData map[string]*openwallet.TxExtractData
	TxID        string
	BlockHeight uint64
	Success     bool
}

//SaveResult 保存结果
type SaveResult struct {
	TxID        string
	BlockHeight uint64
	Success     bool
}

//NewNBlockScanner 创建区块链扫描器
func NewNBlockScanner(wm *WalletManager) *NBlockScanner {
	bs := NBlockScanner{
		BlockScannerBase: openwallet.NewBlockScannerBase(),
	}

	bs.extractingCH = make(chan struct{}, maxExtractingSize)
	bs.wm = wm
	bs.IsScanMemPool = false
	bs.RescanLastBlockCount = 0

	//设置扫描任务
	bs.SetTask(bs.ScanBlockTask)

	return &bs
}

//SetRescanBlockHeight 重置区块链扫描高度
func (bs *NBlockScanner) SetRescanBlockHeight(height uint64) error {
	height = height - 1
	if height < 0 {
		return errors.New("block height to rescan must greater than 0.")
	}

	hash, err := bs.wm.GetBlockHash(height)
	if err != nil {
		return err
	}

	bs.wm.Blockscanner.SaveLocalNewBlock(height, hash)

	return nil
}

//ScanBlockTask 扫描任务
func (bs *NBlockScanner) ScanBlockTask() {

	//获取本地区块高度
	blockHeader, err := bs.GetScannedBlockHeader()
	if err != nil {
		bs.wm.Log.Std.Info("block scanner can not get new block height; unexpected error: %v", err)
		return
	}

	currentHeight := blockHeader.Height
	currentHash := blockHeader.Hash
	var previousHeight uint64 = 0

	for {

		if !bs.Scanning {
			//区块扫描器已暂停，马上结束本次任务
			return
		}

		//获取最大高度
		maxHeight, err := bs.wm.GetBlockHeight()
		if err != nil {
			//下一个高度找不到会报异常
			bs.wm.Log.Std.Info("block scanner can not get rpc-server block height; unexpected error: %v", err)
			break
		}

		//fmt.Println("------[M_block_scan]:    currentHeight: ", currentHeight, "maxHeight: ",maxHeight)
		//是否已到最新高度
		if currentHeight >= maxHeight {
			bs.wm.Log.Std.Info("block scanner has scanned full chain data. Current height: %d", maxHeight)
			break
		}

		//继续扫描下一个区块
		currentHeight = currentHeight + 1
		bs.wm.Log.Std.Info("block scanner scanning height: %d ...", currentHeight)

		var localBlock *Block

		localBlock, err = bs.wm.Client.getBlockByHeight(currentHeight)

		if err != nil {
			//if strings.Contains(err.Error(), "{\"code\":-32000,\"message\":\"Server error\",\"data\":\"DB Not Found Error: BLOCK HEIGHT") {
			//if strings.Contains(err.Error(), "[-32000]{\"name\":\"HANDLER_ERROR\",\"cause\":{\"info\":{},\"name\":\"UNKNOWN_BLOCK\"}") {
			errResp := err.Error()
			if strings.Contains(errResp, "[-32000]") &&
				strings.Contains(errResp, "HANDLER_ERROR") &&
				strings.Contains(errResp, "UNKNOWN_BLOCK") &&
				strings.Contains(errResp, "DB Not Found Error: BLOCK HEIGHT") {
				continue
			} else {
				bs.wm.Log.Std.Info("getBlockByHeight failed; unexpected error: %v", err)
				break
			}
		}

		isFork := false

		//判断hash是否上一区块的hash
		if currentHash != localBlock.PrevBlockHash {
			previousHeight = currentHeight - 1
			bs.wm.Log.Std.Info("block has been fork on height: %d.", currentHeight)
			bs.wm.Log.Std.Info("block height: %d local hash = %s ", previousHeight, currentHash)
			bs.wm.Log.Std.Info("block height: %d mainnet hash = %s ", previousHeight, localBlock.PrevBlockHash)

			bs.wm.Log.Std.Info("delete recharge records on block height: %d.", previousHeight)

			//删除上一区块链的所有充值记录
			//bs.DeleteRechargesByHeight(currentHeight - 1)
			forkBlock, _ := bs.GetLocalBlock(uint32(previousHeight))
			//删除上一区块链的未扫记录
			bs.wm.Blockscanner.DeleteUnscanRecord(uint32(previousHeight))
			currentHeight = previousHeight - 1 //倒退2个区块重新扫描
			if currentHeight <= 0 {
				currentHeight = 1
			}

			localBlock, err = bs.GetLocalBlock(uint32(currentHeight))
			if err != nil && err != storm.ErrNotFound {
				bs.wm.Log.Std.Error("block scanner can not get local block; unexpected error: %v", err)
				break
			} else if err == storm.ErrNotFound {
				//查找core钱包的RPC
				bs.wm.Log.Info("block scanner prev block height:", currentHeight)

				localBlock, err = bs.wm.Client.getBlockByHeight(currentHeight)

				if err != nil {
					//if strings.Contains(err.Error(), "{\"code\":-32000,\"message\":\"Server error\",\"data\":\"DB Not Found Error: BLOCK HEIGHT") {
					//
					//} else {
					bs.wm.Log.Std.Error("block scanner can not get prev block; unexpected error: %v", err)
					break

				}

			}

			//重置当前区块的hash
			currentHash = localBlock.Hash

			bs.wm.Log.Std.Info("rescan block on height: %d, hash: %s .", currentHeight, currentHash)

			//重新记录一个新扫描起点
			bs.wm.Blockscanner.SaveLocalNewBlock(localBlock.Height, localBlock.Hash)

			isFork = true

			if forkBlock != nil {
				//通知分叉区块给观测者，异步处理
				bs.newBlockNotify(forkBlock, isFork)
			}

		} else {

			err = bs.BatchExtractTransaction(localBlock.Height, localBlock.Hash, localBlock.Transactions, false)
			if err != nil {
				bs.wm.Log.Std.Info("block scanner can not extractRechargeRecords; unexpected error: %v", err)
			}

			//重置当前区块的hash
			currentHash = localBlock.Hash

			//保存本地新高度
			bs.wm.Blockscanner.SaveLocalNewBlock(currentHeight, currentHash)
			bs.SaveLocalBlock(localBlock)

			isFork = false
		}

		//通知新区块给观测者，异步处理
		bs.newBlockNotify(localBlock, isFork)
	}

	//重扫前N个块，为保证记录找到
	for i := currentHeight - bs.RescanLastBlockCount; i < currentHeight; i++ {
		bs.scanBlock(i)
	}

	if bs.IsScanMemPool {
		//扫描交易内存池
		bs.ScanTxMemPool()
	}

	//重扫失败区块
	bs.RescanFailedRecord()

}

//ScanBlock 扫描指定高度区块
func (bs *NBlockScanner) ScanBlock(height uint64) error {

	block, err := bs.scanBlock(height)
	if err != nil {
		return err
	}

	bs.newBlockNotify(block, false)

	return nil
}

func (bs *NBlockScanner) scanBlock(height uint64) (*Block, error) {
	var (
		block *Block
		err   error
	)
	block, err = bs.wm.Client.getBlockByHeight(height)

	if err != nil {
		bs.wm.Log.Std.Info("block scanner can not get new block data; unexpected error: %v", err)

		//记录未扫区块
		unscanRecord := openwallet.NewUnscanRecord(height, "", err.Error(), bs.wm.Symbol())
		bs.SaveUnscanRecord(unscanRecord)
		bs.wm.Log.Std.Info("block height: %d extract failed.", height)
		return nil, err
	}

	bs.wm.Log.Std.Info("block scanner scanning height: %d ...", block.Height)

	err = bs.BatchExtractTransaction(block.Height, block.Hash, block.Transactions, false)
	if err != nil {
		bs.wm.Log.Std.Info("block scanner can not extractRechargeRecords; unexpected error: %v", err)
	}

	return block, nil
}

//ScanTxMemPool 扫描交易内存池
func (bs *NBlockScanner) ScanTxMemPool() {

	bs.wm.Log.Std.Info("block scanner scanning mempool ...")

	//提取未确认的交易单
	txIDsInMemPool, err := bs.wm.GetTxIDsInMemPool()
	if err != nil {
		bs.wm.Log.Std.Info("block scanner can not get mempool data; unexpected error: %v", err)
		return
	}

	if len(txIDsInMemPool) == 0 {
		bs.wm.Log.Std.Info("no transactions in mempool ...")
		return
	}

	err = bs.BatchExtractTransaction(0, "", txIDsInMemPool, true)
	if err != nil {
		bs.wm.Log.Std.Info("block scanner can not extractRechargeRecords; unexpected error: %v", err)
	}

}

//rescanFailedRecord 重扫失败记录
func (bs *NBlockScanner) RescanFailedRecord() {

	var (
		blockMap = make(map[uint64][]string)
	)

	list, err := bs.GetUnscanRecords()
	if err != nil {
		bs.wm.Log.Std.Info("block scanner can not get rescan data; unexpected error: %v", err)
	}

	//组合成批处理
	for _, r := range list {

		if _, exist := blockMap[r.BlockHeight]; !exist {
			blockMap[r.BlockHeight] = make([]string, 0)
		}

		if len(r.TxID) > 0 {
			arr := blockMap[r.BlockHeight]
			arr = append(arr, r.TxID)

			blockMap[r.BlockHeight] = arr
		}
	}

	for height, txs := range blockMap {

		var hash string

		if height != 0 {
			bs.wm.Log.Std.Info("block scanner rescanning height: %d ...", height)

			if len(txs) == 0 {

				var block *Block

				block, err = bs.wm.Client.getBlockByHeight(height)

				if err != nil {
					bs.wm.Log.Std.Info("block scanner can not get new block data; unexpected error: %v", err)
					continue
				}

				txs = block.Transactions
			}

			err = bs.BatchExtractTransaction(height, hash, txs, false)
			if err != nil {
				bs.wm.Log.Std.Info("block scanner can not extractRechargeRecords; unexpected error: %v", err)
				continue
			}
		}
		//删除未扫记录
		bs.wm.Blockscanner.DeleteUnscanRecord(uint32(height))
	}

	//删除未没有找到交易记录的重扫记录
	bs.wm.Blockscanner.DeleteUnscanRecordNotFindTX()
}

//newBlockNotify 获得新区块后，通知给观测者
func (bs *NBlockScanner) newBlockNotify(block *Block, isFork bool) {
	header := block.BlockHeader()

	//fmt.Println("M_block_scanner : " + "header.Hash:" + header.Hash + " header.Height:" , header.Height)
	header.Fork = isFork
	bs.NewBlockNotify(header)
}

//BatchExtractTransaction 批量提取交易单
//bitcoin 1M的区块链可以容纳3000笔交易，批量多线程处理，速度更快
func (bs *NBlockScanner) BatchExtractTransaction(blockHeight uint64, blockHash string, txs []string, memPool bool) error {

	var (
		quit       = make(chan struct{})
		done       = 0 //完成标记
		failed     = 0
		shouldDone = len(txs) //需要完成的总数
	)

	if len(txs) == 0 {
		return nil
	}

	//生产通道
	producer := make(chan ExtractResult)
	defer close(producer)

	//消费通道
	worker := make(chan ExtractResult)
	defer close(worker)

	//保存工作
	saveWork := func(height uint64, result chan ExtractResult) {
		//回收创建的地址
		for gets := range result {

			if gets.Success {

				notifyErr := bs.newExtractDataNotify(height, gets.extractData)
				//saveErr := bs.SaveRechargeToWalletDB(height, gets.Recharges)
				if notifyErr != nil {
					failed++ //标记保存失败数
					bs.wm.Log.Std.Info("newExtractDataNotify unexpected error: %v", notifyErr)
				}
			} else {
				//记录未扫区块
				unscanRecord := openwallet.NewUnscanRecord(height, "", "", bs.wm.Symbol())
				bs.SaveUnscanRecord(unscanRecord)
				bs.wm.Log.Std.Info("block height: %d extract failed.", height)
				failed++ //标记保存失败数
			}
			//累计完成的线程数
			done++
			if done == shouldDone {
				//bs.wm.Log.Std.Info("done = %d, shouldDone = %d ", done, len(txs))
				close(quit) //关闭通道，等于给通道传入nil
			}
		}
	}

	//提取工作
	extractWork := func(eblockHeight uint64, eBlockHash string, mTxs []string, eProducer chan ExtractResult) {
		for _, txid := range mTxs {
			bs.extractingCH <- struct{}{}
			//shouldDone++
			go func(mBlockHeight uint64, mTxid string, end chan struct{}, mProducer chan<- ExtractResult) {

				//导出提出的交易
				mProducer <- bs.ExtractTransaction(mBlockHeight, eBlockHash, mTxid, bs.ScanTargetFunc, memPool)
				//释放
				<-end

			}(eblockHeight, txid, bs.extractingCH, eProducer)
		}
	}

	/*	开启导出的线程	*/

	//独立线程运行消费
	go saveWork(blockHeight, worker)

	//独立线程运行生产
	go extractWork(blockHeight, blockHash, txs, producer)

	//以下使用生产消费模式
	bs.extractRuntime(producer, worker, quit)

	if failed > 0 {
		return fmt.Errorf("block scanner saveWork failed")
	} else {
		return nil
	}

	//return nil
}

//extractRuntime 提取运行时
func (bs *NBlockScanner) extractRuntime(producer chan ExtractResult, worker chan ExtractResult, quit chan struct{}) {

	var (
		values = make([]ExtractResult, 0)
	)

	for {
		var activeWorker chan<- ExtractResult
		var activeValue ExtractResult
		//当数据队列有数据时，释放顶部，传输给消费者
		if len(values) > 0 {
			activeWorker = worker
			activeValue = values[0]
		}
		select {
		//生成者不断生成数据，插入到数据队列尾部
		case pa := <-producer:
			values = append(values, pa)
		case <-quit:
			//退出
			//bs.wm.Log.Std.Info("block scanner have been scanned!")
			return
		case activeWorker <- activeValue:
			values = values[1:]
		}
	}
	//return

}

//ExtractTransaction 提取交易单
func (bs *NBlockScanner) ExtractTransaction(blockHeight uint64, blockHash string, txid string, scanAddressFunc openwallet.BlockScanTargetFunc, memPool bool) ExtractResult {

	var (
		result = ExtractResult{
			BlockHeight: blockHeight,
			TxID:        txid,
			extractData: make(map[string]*openwallet.TxExtractData),
			Success:     true,
		}
	)

	//bs.wm.Log.Std.Debug("block scanner scanning tx: %s ...", txid)
	var trx *Transaction
	var err error
	if memPool {
		trx, err = bs.wm.GetTransactionInMemPool(txid)
		if err != nil {
			trx, err = bs.wm.GetTransaction(txid)
			if err != nil {
				bs.wm.Log.Std.Info("block scanner can not extract transaction data in mempool and block chain; unexpected error: %v", err)
				result.Success = false
				return result
			}
		}
	} else {
		trx, err = bs.wm.GetTransaction(txid)

		if err != nil {
			fmt.Println(err.Error())
			fmt.Println(txid)
			if err.Error() == "txnNotFound" {
				trx, err = bs.wm.Client.getTransaction(txid)
				if err != nil {
					bs.wm.Log.Std.Info("block scanner can not extract transaction data; unexpected error: %v", err)
					result.Success = false
					return result
				}
			} else {
				bs.wm.Log.Std.Info("block scanner can not extract transaction data; unexpected error: %v", err)
				result.Success = false
				return result
			}
		}
	}

	//优先使用传入的高度
	if blockHeight > 0 && trx.BlockHeight == 0 {
		trx.BlockHeight = blockHeight
		trx.BlockHash = blockHash
	}

	bs.extractTransaction(trx, &result, bs.ScanTargetFuncV2)

	return result

}

// 从最小单位的 amount 转为带小数点的表示
func convertToAmount(amount *big.Int) string {
	d := decimal.NewFromBigInt(amount, 0)
	w, _ := decimal.NewFromString("1000000000000000000000000")
	d = d.Div(w)
	return d.String()
}

// amount 字符串转为最小单位的表示
func convertFromAmount(amountStr string) *big.Int {
	d, _ := decimal.NewFromString(amountStr)
	w, _ := decimal.NewFromString("1000000000000000000000000")
	d = d.Mul(w)
	r, _ := new(big.Int).SetString(d.String(), 10)
	return r
}

//ExtractTransactionData 提取交易单
func (bs *NBlockScanner) extractTransaction(trx *Transaction, result *ExtractResult, scanAddressFunc openwallet.BlockScanTargetFuncV2) {
	var (
		success = true
	)
	createAt := time.Now().Unix()
	var (
		currentHeight uint64
		err           error
	)

	currentHeight, err = bs.wm.Client.getBlockHeight()

	if trx == nil || err != nil {
		//记录哪个区块哪个交易单没有完成扫描
		success = false
	} else {

		if success {
			//if trx.TxType == "transfer" {
			targetResult := scanAddressFunc(openwallet.ScanTargetParam{
				ScanTarget:     trx.From,
				Symbol:         bs.wm.Symbol(),
				ScanTargetType: openwallet.ScanTargetTypeAccountAddress,
			})

			if targetResult.Exist {

				//if trx.To == "" {
				//	input := openwallet.TxInput{}
				//	input.TxID = trx.TxID
				//	input.Address = trx.From
				//	input.Amount = convertToAmount(trx.Fee)
				//	input.Coin = openwallet.Coin{
				//		Symbol:     bs.wm.Symbol(),
				//		IsContract: false,
				//	}
				//	input.Index = 0
				//	input.Sid = openwallet.GenTxInputSID(trx.TxID, bs.wm.Symbol(), "", uint64(0))
				//	input.CreateAt = createAt
				//	input.BlockHeight = trx.BlockHeight
				//	input.BlockHash = trx.BlockHash
				//	input.Confirm = int64(currentHeight - trx.BlockHeight)
				//	input.IsMemo = true
				//	ed := result.extractData[targetResult.SourceKey]
				//	if ed == nil {
				//		ed = openwallet.NewBlockExtractData()
				//		result.extractData[targetResult.SourceKey] = ed
				//	}
				//} else {
				input := openwallet.TxInput{}
				input.TxID = trx.TxID
				input.Address = trx.From
				input.Amount = convertToAmount(trx.Amount)
				input.Coin = openwallet.Coin{
					Symbol:     bs.wm.Symbol(),
					IsContract: false,
				}
				input.Index = 0
				input.Sid = openwallet.GenTxInputSID(trx.TxID, bs.wm.Symbol(), "", uint64(0))
				input.CreateAt = createAt
				input.BlockHeight = trx.BlockHeight
				input.BlockHash = trx.BlockHash
				input.Confirm = int64(currentHeight - trx.BlockHeight)
				input.IsMemo = true
				ed := result.extractData[targetResult.SourceKey]
				if ed == nil {
					ed = openwallet.NewBlockExtractData()
					result.extractData[targetResult.SourceKey] = ed
				}
				ed.TxInputs = append(ed.TxInputs, &input)

				tmp := *&input
				feeCharge := &tmp
				feeCharge.Index = 1
				feeCharge.Sid = openwallet.GenTxInputSID(trx.TxID, bs.wm.Symbol(), "", uint64(1))
				feeCharge.Amount = convertToAmount(trx.Fee)
				ed.TxInputs = append(ed.TxInputs, feeCharge)

				//}
			}
			//} else {
			//	targetResult := scanAddressFunc(openwallet.ScanTargetParam {
			//		ScanTarget:     trx.From,
			//		Symbol:         bs.wm.Symbol(),
			//		ScanTargetType: openwallet.ScanTargetTypeAccountAddress,
			//	})
			//
			//
			//	if targetResult.Exist {
			//
			//
			//		input := openwallet.TxInput{}
			//		input.TxID = trx.TxID
			//		input.Address = trx.From
			//		input.Amount = "convertToAmount(trx.Fee)"
			//		input.Coin = openwallet.Coin{
			//			Symbol:     bs.wm.Symbol(),
			//			IsContract: false,
			//		}
			//		input.Index = 0
			//		input.Sid = openwallet.GenTxInputSID(trx.TxID, bs.wm.Symbol(), "", uint64(0))
			//		input.CreateAt = createAt
			//		input.BlockHeight = trx.BlockHeight
			//		input.BlockHash = trx.BlockHash
			//		input.Confirm = int64(currentHeight - trx.BlockHeight)
			//		input.IsMemo = true
			//		ed := result.extractData[targetResult.SourceKey]
			//		if ed == nil {
			//			ed = openwallet.NewBlockExtractData()
			//			result.extractData[targetResult.SourceKey] = ed
			//		}
			//		ed.TxInputs = append(ed.TxInputs, &input)
			//	}
			//}

			if trx.TxType == "transfer" {
				targetResult = scanAddressFunc(openwallet.ScanTargetParam{
					ScanTarget:     trx.To,
					Symbol:         bs.wm.Symbol(),
					ScanTargetType: openwallet.ScanTargetTypeAccountAddress,
				})

				if targetResult.Exist {

					output := openwallet.TxOutPut{}
					output.TxID = trx.TxID
					output.Address = trx.To
					output.Amount = convertToAmount(trx.Amount)
					output.Coin = openwallet.Coin{
						Symbol:     bs.wm.Symbol(),
						IsContract: false,
					}
					output.Index = 0
					output.Sid = openwallet.GenTxOutPutSID(trx.TxID, bs.wm.Symbol(), "", 0)
					output.CreateAt = createAt
					output.BlockHeight = trx.BlockHeight
					output.BlockHash = trx.BlockHash
					output.Confirm = int64(currentHeight - trx.BlockHeight)
					output.IsMemo = true
					ed := result.extractData[targetResult.SourceKey]
					if ed == nil {
						ed = openwallet.NewBlockExtractData()
						result.extractData[targetResult.SourceKey] = ed
					}

					ed.TxOutputs = append(ed.TxOutputs, &output)
				}
			}

			for _, extractData := range result.extractData {
				status := ""
				if trx.Status == "" {
					status = "1"
				} else {
					status = "0"
					//bs.wm.Log.Std.Info("newExtractDataNotify unexpected error: %v", "[M:wrong_status] txid-" + trx.TxID + " status-" + trx.Status)
					//fmt.Println("[M:wrong_status] txid-", trx.TxID, " status-", trx.Status)
				}
				tx := &openwallet.Transaction{
					From:   []string{trx.From + ":" + convertToAmount(trx.Amount)},
					To:     []string{trx.To + ":" + convertToAmount(trx.Amount)},
					Amount: convertToAmount(trx.Amount),
					Fees:   convertToAmount(trx.Fee),
					Coin: openwallet.Coin{
						Symbol:     bs.wm.Symbol(),
						IsContract: false,
					},
					BlockHash:   trx.BlockHash,
					BlockHeight: trx.BlockHeight,
					TxID:        trx.TxID,
					Decimal:     6,
					Status:      status,
					SubmitTime:  int64(trx.TimeStamp),
					ConfirmTime: int64(trx.TimeStamp),
					IsMemo:      true,
				}

				//tx.SetExtParam("memo", trx.DestinationTag)
				//tx.SetExtParam("memo", trx.Memo)
				//
				//if !(trx.TxType == "Payment" && trx.To != "") {
				//	tx.From = []string{trx.From + ":" + "convertToAmount(trx.Fee)"}
				//	tx.To = []string{trx.To + ":" + "convertToAmount(trx.Fee)"}
				//	tx.Amount = "convertToAmount(trx.Fee)"
				//	tx.Fees = "0"
				//}

				wxID := openwallet.GenTransactionWxID(tx)
				tx.WxID = wxID
				extractData.Transaction = tx
			}

		}

		success = true

	}
	result.Success = success
}

//newExtractDataNotify 发送通知
func (bs *NBlockScanner) newExtractDataNotify(height uint64, extractData map[string]*openwallet.TxExtractData) error {

	for o, _ := range bs.Observers {
		for key, data := range extractData {
			err := o.BlockExtractDataNotify(key, data)
			if err != nil {
				bs.wm.Log.Error("BlockExtractDataNotify unexpected error:", err)
				//记录未扫区块
				unscanRecord := openwallet.NewUnscanRecord(height, "", "ExtractData Notify failed.", bs.wm.Symbol())
				err = bs.SaveUnscanRecord(unscanRecord)
				if err != nil {
					bs.wm.Log.Std.Error("block height: %d, save unscan record failed. unexpected error: %v", height, err.Error())
				}

			}
		}
	}

	return nil
}

//DeleteUnscanRecordNotFindTX 删除未没有找到交易记录的重扫记录
func (bs *NBlockScanner) DeleteUnscanRecordNotFindTX() error {

	//删除找不到交易单
	reason := "[-5]No information available about transaction"

	if bs.BlockchainDAI == nil {
		return fmt.Errorf("Blockchain DAI is not setup ")
	}

	list, err := bs.BlockchainDAI.GetUnscanRecords(bs.wm.Symbol())
	if err != nil {
		return err
	}

	for _, r := range list {
		if strings.HasPrefix(r.Reason, reason) {
			bs.BlockchainDAI.DeleteUnscanRecordByID(r.ID, bs.wm.Symbol())
		}
	}
	return nil
}

//SaveRechargeToWalletDB 保存交易单内的充值记录到钱包数据库
//func (bs *NBlockScanner ) SaveRechargeToWalletDB(height uint64, list []*openwallet.Recharge) error {
//
//	for _, r := range list {
//
//		//accountID := "W4ruoAyS5HdBMrEeeHQTBxo4XtaAixheXQ"
//		wallet, ok := bs.GetWalletByAddress(r.Address)
//		if ok {
//
//			//a := wallet.GetAddress(r.Address)
//			//if a == nil {
//			//	continue
//			//}
//			//
//			//r.AccountID = a.AccountID
//
//			err := wallet.SaveUnreceivedRecharge(r)
//			//如果blockHash没有值，添加到重扫，避免遗留
//			if err != nil || len(r.BlockHash) == 0 {
//
//				//记录未扫区块
//				unscanRecord := NewUnscanRecord(height, r.TxID, "save to wallet failed.")
//				err = bs.SaveUnscanRecord(unscanRecord)
//				if err != nil {
//					bs.wm.Log.Std.Error("block height: %d, txID: %s save unscan record failed. unexpected error: %v", height, r.TxID, err.Error())
//				}
//
//			} else {
//				bs.wm.Log.Info("block scanner save blockHeight:", height, "txid:", r.TxID, "address:", r.Address, "successfully.")
//			}
//		} else {
//			return errors.New("address in wallet is not found")
//		}
//
//	}
//
//	return nil
//}

//GetCurrentBlockHeader 获取全网最新高度区块头
func (bs *NBlockScanner) GetCurrentBlockHeader() (*openwallet.BlockHeader, error) {
	var (
		blockHeight uint64 = 0
		err         error
	)

	blockHeight, err = bs.wm.GetBlockHeight()
	if err != nil {
		return nil, err
	}
	var block *Block

	block, err = bs.wm.Client.getBlockByHeight(blockHeight)

	if err != nil {
		bs.wm.Log.Errorf("get block spec by block number failed, err=%v", err)
		return nil, err
	}

	return &openwallet.BlockHeader{Height: blockHeight, Hash: block.Hash}, nil
}

//GetScannedBlockHeader 获取已扫高度区块头
func (bs *NBlockScanner) GetScannedBlockHeader() (*openwallet.BlockHeader, error) {

	var (
		blockHeight uint64 = 0
		hash        string
		err         error
	)

	blockHeight, hash, err = bs.wm.Blockscanner.GetLocalNewBlock()
	if err != nil {
		bs.wm.Log.Errorf("get local new block failed, err=%v", err)
		return nil, err
	}

	//如果本地没有记录，查询接口的高度
	if blockHeight == 0 {
		blockHeight, err = bs.wm.GetBlockHeight()
		if err != nil {
			bs.wm.Log.Errorf("M GetBlockHeight failed,err = %v", err)
			return nil, err
		}

		//就上一个区块链为当前区块
		blockHeight = blockHeight - 1
		var block *Block

		block, err = bs.wm.Client.getBlockByHeight(blockHeight)

		if err != nil {
			bs.wm.Log.Errorf("get block spec by block number failed, err=%v", err)
			return nil, err
		}

		hash = block.Hash
	}

	return &openwallet.BlockHeader{Height: blockHeight, Hash: hash}, nil
}

//GetScannedBlockHeight 获取已扫区块高度
func (bs *NBlockScanner) GetScannedBlockHeight() uint64 {
	localHeight, _, _ := bs.wm.Blockscanner.GetLocalNewBlock()
	return localHeight
}

func (bs *NBlockScanner) ExtractTransactionData(txid string, scanTargetFunc openwallet.BlockScanTargetFunc) (map[string][]*openwallet.TxExtractData, error) {

	scanAddressFunc := func(t openwallet.ScanTarget) (string, bool) {
		sourceKey, ok := scanTargetFunc(openwallet.ScanTarget{
			Address:          t.Address,
			Symbol:           bs.wm.Symbol(),
			BalanceModelType: bs.wm.BalanceModelType(),
		})
		return sourceKey, ok

	}
	result := bs.ExtractTransaction(0, "", txid, scanAddressFunc, false)
	if !result.Success {
		return nil, fmt.Errorf("extract transaction failed")
	}
	extData := make(map[string][]*openwallet.TxExtractData)
	for key, data := range result.extractData {
		txs := extData[key]
		if txs == nil {
			txs = make([]*openwallet.TxExtractData, 0)
		}
		txs = append(txs, data)
		extData[key] = txs
	}
	return extData, nil
}

//DropRechargeRecords 清楚钱包的全部充值记录
//func (bs *NBlockScanner ) DropRechargeRecords(accountID string) error {
//	bs.mu.RLock()
//	defer bs.mu.RUnlock()
//
//	wallet, ok := bs.walletInScanning[accountID]
//	if !ok {
//		errMsg := fmt.Sprintf("accountID: %s wallet is not found", accountID)
//		return errors.New(errMsg)
//	}
//
//	return wallet.DropRecharge()
//}

//DeleteRechargesByHeight 删除某区块高度的充值记录
//func (bs *NBlockScanner ) DeleteRechargesByHeight(height uint64) error {
//
//	bs.mu.RLock()
//	defer bs.mu.RUnlock()
//
//	for _, wallet := range bs.walletInScanning {
//
//		list, err := wallet.GetRecharges(false, height)
//		if err != nil {
//			return err
//		}
//
//		db, err := wallet.OpenDB()
//		if err != nil {
//			return err
//		}
//
//		tx, err := db.Begin(true)
//		if err != nil {
//			return err
//		}
//
//		for _, r := range list {
//			err = db.DeleteStruct(&r)
//			if err != nil {
//				return err
//			}
//		}
//
//		tx.Commit()
//
//		db.Close()
//	}
//
//	return nil
//}

//GetSourceKeyByAddress 获取地址对应的数据源标识
func (bs *NBlockScanner) GetSourceKeyByAddress(address string) (string, bool) {
	bs.Mu.RLock()
	defer bs.Mu.RUnlock()

	sourceKey, ok := bs.AddressInScanning[address]
	return sourceKey, ok
}

//GetWalletByAddress 获取地址对应的钱包
// func (bs *NBlockScanner ) GetWalletByAddress(address string) (*openwallet.Wallet, bool) {
// 	bs.mu.RLock()
// 	defer bs.mu.RUnlock()

// 	account, ok := bs.addressInScanning[address]
// 	if ok {
// 		wallet, ok := bs.walletInScanning[account]
// 		return wallet, ok

// 	} else {
// 		return nil, false
// 	}
// }

//GetBlockHeight 获取区块链高度
func (wm *WalletManager) GetBlockHeight() (uint64, error) {
	return wm.Client.getBlockHeight()
}

//GetLocalNewBlock 获取本地记录的区块高度和hash
func (bs *NBlockScanner) GetLocalNewBlock() (uint64, string, error) {

	if bs.BlockchainDAI == nil {
		return 0, "", fmt.Errorf("Blockchain DAI is not setup ")
	}

	header, err := bs.BlockchainDAI.GetCurrentBlockHead(bs.wm.Symbol())
	if err != nil {
		return 0, "", err
	}

	return header.Height, header.Hash, nil
}

//SaveLocalNewBlock 记录区块高度和hash到本地
func (bs *NBlockScanner) SaveLocalNewBlock(blockHeight uint64, blockHash string) error {

	if bs.BlockchainDAI == nil {
		return fmt.Errorf("Blockchain DAI is not setup ")
	}

	header := &openwallet.BlockHeader{
		Hash:   blockHash,
		Height: blockHeight,
		Fork:   false,
		Symbol: bs.wm.Symbol(),
	}

	return bs.BlockchainDAI.SaveCurrentBlockHead(header)
}

//GetBlockHash 根据区块高度获得区块hash
func (wm *WalletManager) GetBlockHash(height uint64) (string, error) {
	return wm.Client.getBlockHash(height)
}

//GetBlock 获取区块数据
func (wm *WalletManager) GetBlock(hash string) (*Block, error) {
	return wm.Client.getBlock(hash)
}

//GetTxIDsInMemPool 获取待处理的交易池中的交易单IDs
func (wm *WalletManager) GetTxIDsInMemPool() ([]string, error) {
	return nil, nil
}

func (wm *WalletManager) GetTransactionInMemPool(txid string) (*Transaction, error) {
	return nil, nil
}

//GetTransaction 获取交易单
func (wm *WalletManager) GetTransaction(txid string) (*Transaction, error) {
	return wm.Client.getTransaction(txid)
}

//GetAssetsAccountBalanceByAddress 查询账户相关地址的交易记录
func (bs *NBlockScanner) GetBalanceByAddress(address ...string) ([]*openwallet.Balance, error) {

	addrsBalance := make([]*openwallet.Balance, 0)

	for _, addr := range address {
		var (
			balance *AddrBalance
			err     error
		)

		balance, err = bs.wm.Client.getBalance(addr)

		if err != nil {
			return nil, err
		}

		addrsBalance = append(addrsBalance, &openwallet.Balance{
			Symbol:  bs.wm.Symbol(),
			Address: addr,
			Balance: convertToAmount(balance.Balance),
		})
	}

	return addrsBalance, nil
}

func (c *Client) getMultiAddrTransactions(memoScan string, offset, limit int, addresses ...string) ([]*Transaction, error) {
	var (
		trxs      = make([]*Transaction, 0)
		respLimit = "/limit/10000"
	)

	for _, addr := range addresses {
		path := "transactions/address/" + addr + respLimit

		resp, err := c.Call(path, nil)
		if err != nil {
			return nil, err
		}
		txArray := resp.Array()[0].Array()

		for _, txDetail := range txArray {
			trxs = append(trxs, c.NewTransaction(&txDetail))
		}
	}

	return trxs, nil
}

//GetAssetsAccountTransactionsByAddress 查询账户相关地址的交易记录
func (bs *NBlockScanner) GetTransactionsByAddress(offset, limit int, coin openwallet.Coin, address ...string) ([]*openwallet.TxExtractData, error) {
	//
	//var (
	//	array = make([]*openwallet.TxExtractData, 0)
	//)
	//
	//trxs, err := bs.wm.Client.getMultiAddrTransactions(bs.wm.Config.MemoScan, offset, limit, address...)
	//if err != nil {
	//	return nil, err
	//}
	//
	//key := "account"
	//
	////提取账户相关的交易单
	//var scanAddressFunc openwallet.BlockScanAddressFunc = func(findAddr string) (string, bool) {
	//	for _, a := range address {
	//		if findAddr == a {
	//			return key, true
	//		}
	//	}
	//	return "", false
	//}
	//
	////要检查一下tx.BlockHeight是否有值
	//
	//for _, tx := range trxs {
	//
	//	result := ExtractResult{
	//		BlockHeight: tx.BlockHeight,
	//		TxID:        tx.TxID,
	//		extractData: make(map[string]*openwallet.TxExtractData),
	//		Success:     true,
	//	}
	//
	//	bs.extractTransaction(tx, &result, bs.ScanTargetFuncV2)
	//	data := result.extractData
	//	txExtract := data[key]
	//	if txExtract != nil {
	//		array = append(array, txExtract)
	//	}
	//
	//}
	//
	//return array, nil
	return nil, nil
}

//Run 运行
func (bs *NBlockScanner) Run() error {

	bs.BlockScannerBase.Run()

	return nil
}

////Stop 停止扫描
func (bs *NBlockScanner) Stop() error {

	bs.BlockScannerBase.Stop()

	return nil
}

//Pause 暂停扫描
func (bs *NBlockScanner) Pause() error {

	bs.BlockScannerBase.Pause()

	return nil
}

//Restart 继续扫描
func (bs *NBlockScanner) Restart() error {

	bs.BlockScannerBase.Restart()

	return nil
}

/******************* 使用insight socket.io 监听区块 *******************/

//setupSocketIO 配置socketIO监听新区块
func (bs *NBlockScanner) setupSocketIO() error {

	bs.wm.Log.Info("block scanner use socketIO to listen new data")

	var (
		room = "inv"
	)

	if bs.socketIO == nil {

		apiUrl, err := url.Parse(bs.wm.Config.ServerAPI)
		if err != nil {
			return err
		}
		domain := apiUrl.Hostname()
		port := common.NewString(apiUrl.Port()).Int()
		c, err := gosocketio.Dial(
			gosocketio.GetUrl(domain, port, false),
			transport.GetDefaultWebsocketTransport())
		if err != nil {
			return err
		}

		bs.socketIO = c

	}

	err := bs.socketIO.On("tx", func(h *gosocketio.Channel, args interface{}) {
		//bs.wm.Log.Info("block scanner socketIO get new transaction received: ", args)
		txMap, ok := args.(map[string]interface{})
		if ok {
			txid := txMap["txid"].(string)
			errInner := bs.BatchExtractTransaction(0, "", []string{txid}, false)
			if errInner != nil {
				bs.wm.Log.Std.Info("block scanner can not extractRechargeRecords; unexpected error: %v", errInner)
			}
		}

	})
	if err != nil {
		return err
	}

	/*
		err = bs.socketIO.On("block", func(h *gosocketio.Channel, args interface{}) {
			bs.wm.Log.Info("block scanner socketIO get new block received: ", args)
			hash, ok := args.(string)
			if ok {

				block, errInner := bs.wm.GetBlock(hash)
				if errInner != nil {
					bs.wm.Log.Std.Info("block scanner can not get new block data; unexpected error: %v", errInner)
				}

				errInner = bs.scanBlock(block)
				if errInner != nil {
					bs.wm.Log.Std.Info("block scanner can not block: %d; unexpected error: %v", block.Height, errInner)
				}
			}

		})
		if err != nil {
			return err
		}
	*/

	err = bs.socketIO.On(gosocketio.OnDisconnection, func(h *gosocketio.Channel) {
		bs.wm.Log.Info("block scanner socketIO disconnected")
	})
	if err != nil {
		return err
	}

	err = bs.socketIO.On(gosocketio.OnConnection, func(h *gosocketio.Channel) {
		bs.wm.Log.Info("block scanner socketIO connected")
		h.Emit("subscribe", room)
	})
	if err != nil {
		return err
	}

	return nil
}

//SupportBlockchainDAI 支持外部设置区块链数据访问接口
//@optional
func (bs *NBlockScanner) SupportBlockchainDAI() bool {
	return true
}
