package nearTransaction

import (
	"encoding/hex"
	"errors"
	"math/big"
)

type Account struct {
	Length []byte
	ID     []byte
}

func NewAccount(ID string) *Account {
	//if len(ID) == 32{
	//	idBytes, _ := hex.DecodeString(ID)
	//	return &Account{
	//		Length: uint32ToLittleEndianBytes(64),
	//		ID:     idBytes,
	//	}
	//}
	return &Account{
		Length: uint32ToLittleEndianBytes(uint32(len(ID))),
		ID:     []byte(ID),
	}
}

func (a *Account) ToBytes () []byte {return append(a.Length, a.ID...)}

type PublicKey struct {
	KeyType byte
	Key     []byte
}

func NewPublicKey(key string) (*PublicKey, error) {
	if len(key) != 64 {
		return nil, errors.New("public key length error")
	}

	keyBytes, err := hex.DecodeString(key)
	if err != nil {
		return nil, errors.New("invalid public key")
	}

	return &PublicKey{
		KeyType: KeyTypeED25519,
		Key:     keyBytes,
	}, nil
}

func (p *PublicKey) ToBytes () []byte {return append([]byte{p.KeyType}, p.Key...)}

type Action struct {
	ActionType byte
	Amount     []byte
}

func NewAmount(bigAmount *big.Int) []byte {
	amount := reverseBytes(bigAmount.Bytes())

	offset := 16 - len(amount)

	for i := 0; i < offset; i ++ {
		amount = append(amount, 0)
	}

	return amount
}

func (a *Action) ToBytes () []byte {return append([]byte{a.ActionType}, a.Amount...)}

type TxStruct struct {
	Signer *Account
	SignerPublicKey *PublicKey
	Nonce []byte
	Receiver *Account
	BlockHash []byte
	Actions []Action
}

func (tx *Transfer) NewTxStruct() (*TxStruct, error) {
	var ts TxStruct
	if !IsValid(tx.SignerID) {
		return nil, errors.New("invalid signer ID")
	}
	ts.Signer = NewAccount(tx.SignerID)

	publicKey, err := NewPublicKey(tx.SignerPublicKey)
	if err != nil {
		return nil, err
	}
	ts.SignerPublicKey = publicKey

	ts.Nonce = uint64ToLittleEndianBytes(tx.Nonce)

	if !IsValid(tx.ReceiverID) {
		return nil, errors.New("invalid receiver ID")
	}
	ts.Receiver = NewAccount(tx.ReceiverID)

	hashBytes, err := Decode(tx.RecentBlockHash, BitcoinAlphabet)
	if err != nil || len(hashBytes) != 32 {
		return nil, errors.New("invalid recent block hash")
	}
	ts.BlockHash = hashBytes

	if tx.AmountInYoctoN.Cmp(big.NewInt(0)) > 0 {
		ts.Actions = append(ts.Actions, Action{
			ActionType: ActionTransfer,
			Amount:     NewAmount(tx.AmountInYoctoN),
		})
	}

	return &ts, nil
}

func (tx *TxStruct) ToBytes() []byte {
	txBytes := make([]byte, 0)
	txBytes = append(txBytes, tx.Signer.ToBytes()...)
	txBytes = append(txBytes, tx.SignerPublicKey.ToBytes()...)
	txBytes = append(txBytes, tx.Nonce...)
	txBytes = append(txBytes, tx.Receiver.ToBytes()...)
	txBytes = append(txBytes, tx.BlockHash...)

	if tx.Actions == nil || len(tx.Actions) == 0 {
		txBytes = append(txBytes, uint32ToLittleEndianBytes(1)...)
		txBytes = append(txBytes, 0)
	} else {
		txBytes = append(txBytes, uint32ToLittleEndianBytes(uint32(len(tx.Actions)))...)
		for _, action := range tx.Actions {
			txBytes = append(txBytes, action.ToBytes()...)
		}
	}

	return txBytes
}