package nearTransaction

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"
)

func TestTransfer(t *testing.T) {
	signerID := "sender.testnet"
	signerPublicKey := "bc7bc2614fafe07798872abc0e25770f393e10c1a893f96cdf2890ce290bc35e"
	nonce := uint64(1)
	receiverID := "receiver.testnet"
	recentBlockHash := "4EZn16JrHvB52A8G4JzkYn6RgDRt8z9FcLGYftb8QUFu"
	privateKey, _ := hex.DecodeString("e0c0c1a43f521f32c05a647a3595304c4992c9344f03eb66b61c796052001775")
	amount, _ := new(big.Int).SetString("1000000000000000000000000", 10)

	transfer := NewTransfer(signerID, signerPublicKey, nonce,receiverID, recentBlockHash, amount)

	emptytrans, hash, err := transfer.CreateEmptyTransactionAndHash()
	if err != nil {
		fmt.Println(err)
		return
	} else {
		fmt.Println("empty : ", emptytrans)
		fmt.Println("hash  : ", hash)
	}

	sig, err := SignTransaction(hash, privateKey)
	if err != nil {
		fmt.Println(err)
		return
	} else {
		fmt.Println("signature : ", hex.EncodeToString(sig))
	}

	signedTrans, pass := VerifyAndCombineTransaction(emptytrans, hash, signerPublicKey, hex.EncodeToString(sig))
	if pass {
		fmt.Println("signed : ", signedTrans)
	} else {
		t.Error("failed")
	}
}
