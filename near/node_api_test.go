package near

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

const (
	testNodeAPI = "https://rpc.mainnet.near.org/"

)

func Test_getBlockHeight(t *testing.T) {

	c := NewClient(testNodeAPI, true)

	r, err := c.getBlockHeight()

	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println("height:", r)
	}
}

func Test_getNonce(t *testing.T) {
	address := "95cc0306c93744612e7986f6a3cc1e091a95a1bcb51cdd8885b35e0eb6229527"
	c := NewClient(testNodeAPI, true)
	r, err := c.getNonce(address)
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(r)
	}
}

func Test_getBlockByHeight(t *testing.T) {

	c := NewClient(testNodeAPI, true)
	r, err := c.getBlockByHeight(20525812)//114853
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(r)
	}
}


func Test_getBlockHash(t *testing.T) {

	c := NewClient(testNodeAPI, true)

	height := uint64(20634163)

	r, err := c.getBlockHash(height)

	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(r)
	}

}

func Test_getGasPrice(t *testing.T) {
	c := NewClient(testNodeAPI, true)

	c.getGasPrice()
}

func Test_getAccess(t *testing.T) {
	c := NewClient(testNodeAPI, true)
	//address := "d3539d5e972dc51e8bb2f023c9499394a9325710e19a09c7c1c9afc71053f7f4"
	address := "71a2caf1b6f369d64dc2ca2db950d7b296bfbe958a416fa294a83dbcd2cbe6f1"

	c.getAccess(address)
}

func Test_getBalance(t *testing.T) {

	c := NewClient(testNodeAPI, true)
	//address := "d3539d5e972dc51e8bb2f023c9499394a9325710e19a09c7c1c9afc71053f7f4"
	address := "71a2caf1b6f369d64dc2ca2db950d7b296bfbe958a416fa294a83dbcd2cbe6f1"

	r, err := c.getBalance(address)

	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(r.Balance.String())
	}
}

func Test_getChunk(t *testing.T) {
	c := NewClient(testNodeAPI, true)

	chunk := "HPJ95VF6YArbSkkqpaJUnqLnE8LoVxHcUwmb9XW2itLZ"

	r, err := c.getTransactionsInChunks(chunk)

	fmt.Println(err)
	fmt.Println(r)
}

func Test_getTransaction(t *testing.T) {

	c := NewClient(testNodeAPI, true)
	txid := "FFfHgQNkysH3x9NZowNtLoDeMzpADpmAos1hFo4WUNhw"
	r, err := c.getTransaction(txid)

	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(r)
	}

	txid = "63pwAr4X3wq8otjvaAqbHuGC7p1JF29z5D5SbTEW298V"
	r, err = c.getTransaction(txid)

	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(r)
	}
}

func Test_convert(t *testing.T) {

	amount := uint64(5000000001)

	amountStr := fmt.Sprintf("%d", amount)

	fmt.Println(amountStr)

	d, _ := decimal.NewFromString(amountStr)

	w, _ := decimal.NewFromString("100000000")

	d = d.Div(w)

	fmt.Println(d.String())

	d = d.Mul(w)

	fmt.Println(d.String())

	r, _ := strconv.ParseInt(d.String(), 10, 64)

	fmt.Println(r)

	fmt.Println(time.Now().UnixNano())
}

func Test_getTransactionByAddresses(t *testing.T) {
	addrs := "ARAA8AnUYa4kWwWkiZTTyztG5C6S9MFTx11"

	c := NewClient(testNodeAPI, true)
	result, err := c.getMultiAddrTransactions("MemoData", 0, -1, addrs)

	if err != nil {
		t.Error("get transactions failed!")
	} else {
		for _, tx := range result {
			fmt.Println(tx.TxID)
		}
	}
}

func Test_tmp(t *testing.T) {

	c := NewClient(testNodeAPI, true)

	block, err := c.getBlockByHeight(48059631)
	if err != nil {
		t.Error(err)
	}
	fmt.Println(block)
}
