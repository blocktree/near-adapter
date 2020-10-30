package nearTransaction

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"github.com/blocktree/go-owcrypt"
	"math/big"
	"strings"
)

type Transfer struct {
	SignerID string
	SignerPublicKey string
	Nonce uint64
	ReceiverID string
	RecentBlockHash string
	AmountInYoctoN *big.Int
}

func NewTransfer(signerID, signerPublicKey string, nonce uint64, reveiverID, recentBlockHash string, amountInYoctoN *big.Int) *Transfer {
	return &Transfer{
		SignerID:        signerID,
		SignerPublicKey: signerPublicKey,
		Nonce:           nonce,
		ReceiverID:      reveiverID,
		RecentBlockHash: recentBlockHash,
		AmountInYoctoN:  amountInYoctoN,
	}
}

func (tx *Transfer) CreateEmptyTransactionAndHash() (string, string, error) {
	ts, err := tx.NewTxStruct()
	if err != nil {
		return "", "", err
	}

	emptyTrans := ts.ToBytes()

	return hex.EncodeToString(emptyTrans),
		hex.EncodeToString(owcrypt.Hash(emptyTrans, 0, owcrypt.HASH_ALG_SHA256)),
		nil

}

func SignTransaction(hash string, privateKey []byte) ([]byte, error) {
	if privateKey == nil || len(privateKey) != 32 {
		return nil, errors.New("invalid private key")
	}

	hashBytes, err := hex.DecodeString(hash)
	if err != nil || len(hashBytes) != 32 {
		return nil, errors.New("invalid transaction hash")
	}

	signature, _, retCode := owcrypt.Signature(privateKey, nil, hashBytes, owcrypt.ECC_CURVE_ED25519)
	if retCode != owcrypt.SUCCESS {
		return nil, errors.New("sign failed" )
	}

	return signature, nil
}

func VerifyAndCombineTransaction(emptyTrans, hash, publicKey, signature string) (string, bool) {
	trans, err := hex.DecodeString(emptyTrans)
	if err != nil || len(trans) == 0 {
		return "", false
	}
	hashBytes := owcrypt.Hash(trans, 0, owcrypt.HASH_ALG_SHA256)
	if hex.EncodeToString(hashBytes) != strings.ToLower(hash) {
		return "", false
	}

	pubBytes, err := hex.DecodeString(publicKey)
	if err != nil || len(pubBytes) != 32 {
		return "", false
	}

	sigBytes, err := hex.DecodeString(signature)
	if err != nil || len(sigBytes) != 64 {
		return "", false
	}

	if owcrypt.SUCCESS != owcrypt.Verify(pubBytes, nil, hashBytes, sigBytes, owcrypt.ECC_CURVE_ED25519) {
		return "", false
	}

	signedTrans, _ := hex.DecodeString(emptyTrans + hex.EncodeToString([]byte{KeyTypeED25519}) + signature)

	return base64.StdEncoding.EncodeToString(signedTrans), true
}