package internal

import (
	"context"
	"fmt"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/google/uuid"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	xauthsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"google.golang.org/grpc"
)

type TransactionMaker struct {
	grpcConn         *grpc.ClientConn
	codec            *codec.ProtoCodec
	txConfig         client.TxConfig
	txBuilder        client.TxBuilder
	senderAcc        types.AccAddress
	senderAccBase    *authtypes.BaseAccount
	senderPrivateKey cryptotypes.PrivKey
	receiverAcc      types.AccAddress
	chainID          string
	kring            keyring.Keyring
}

func NewTransactionMaker(grpcConn *grpc.ClientConn, chainID, senderAddress, receiverAddress, passphrase string, privateKey []byte) (*TransactionMaker, error) {
	tm := new(TransactionMaker)
	tm.grpcConn = grpcConn

	var err error
	tm.senderAcc, err = types.AccAddressFromBech32(senderAddress)
	if err != nil {
		return nil, err
	}

	tm.receiverAcc, err = types.AccAddressFromBech32(receiverAddress)
	if err != nil {
		return nil, err
	}

	tm.senderAccBase, err = tm.GetAccountInfo(tm.senderAcc.String())
	if err != nil {
		return nil, err
	}

	interfaceRegistry := codectypes.NewInterfaceRegistry()
	interfaceRegistry.RegisterInterface("types.PubKey", (*cryptotypes.PubKey)(nil), &secp256k1.PubKey{})
	interfaceRegistry.RegisterInterface("types.Msg", (*types.Msg)(nil), &banktypes.MsgSend{})
	tm.codec = codec.NewProtoCodec(interfaceRegistry)
	tm.txConfig = authtx.NewTxConfig(tm.codec, authtx.DefaultSignModes)
	tm.txBuilder = tm.txConfig.NewTxBuilder()
	tm.chainID = chainID

	tm.kring = keyring.NewInMemory(tm.codec)
	err = tm.kring.ImportPrivKey(uuid.New().String(), string(privateKey), passphrase)
	if err != nil {
		return nil, err
	}

	return tm, nil
}

func (tm *TransactionMaker) BuildTx(gasLimit uint64, amount, feeAmount int64, memo string) error {
	message := banktypes.NewMsgSend(
		tm.senderAcc,
		tm.receiverAcc,
		types.NewCoins(types.NewInt64Coin("uatom", amount)),
	)
	err := tm.txBuilder.SetMsgs(message)
	if err != nil {
		return err
	}

	tm.txBuilder.SetGasLimit(gasLimit)
	tm.txBuilder.SetMemo(memo)
	tm.txBuilder.SetFeeAmount(types.NewCoins(types.NewInt64Coin("uatom", feeAmount)))

	return nil
}

func (tm *TransactionMaker) SignTx() error {
	err := tm.txBuilder.SetSignatures(signing.SignatureV2{
		PubKey: tm.senderAccBase.GetPubKey(),
		Data: &signing.SingleSignatureData{
			SignMode:  tm.txConfig.SignModeHandler().DefaultMode(),
			Signature: nil,
		},
		Sequence: tm.senderAccBase.GetSequence(),
	})

	if err != nil {
		return err
	}

	signerData := xauthsigning.SignerData{
		Address:       tm.senderAcc.String(),
		ChainID:       tm.chainID,
		AccountNumber: tm.senderAccBase.GetAccountNumber(),
		Sequence:      tm.senderAccBase.GetSequence(),
		PubKey:        tm.senderAccBase.GetPubKey(),
	}

	var sigV2 signing.SignatureV2

	signMode := tm.txConfig.SignModeHandler().DefaultMode()
	signBytes, err := tm.txConfig.SignModeHandler().GetSignBytes(signMode, signerData, tm.txBuilder.GetTx())
	if err != nil {
		return err
	}

	sig, _, err := tm.kring.SignByAddress(tm.senderAcc, signBytes)
	if err != nil {
		return err
	}

	sigData := signing.SingleSignatureData{
		SignMode:  signMode,
		Signature: sig,
	}

	sigV2 = signing.SignatureV2{
		PubKey:   tm.senderAccBase.GetPubKey(),
		Data:     &sigData,
		Sequence: signerData.Sequence,
	}

	err = tm.txBuilder.SetSignatures(sigV2)

	return err
}

func (tm *TransactionMaker) BroadcastTx() (*tx.BroadcastTxResponse, error) {
	txBytes, err := tm.txConfig.TxEncoder()(tm.txBuilder.GetTx())
	if err != nil {
		return nil, err
	}

	// JSON String (not required, just showing for reference)
	qeqweqasd := tm.txBuilder.GetTx()
	txBytesJson, err := tm.txConfig.TxJSONEncoder()(qeqweqasd)
	if err != nil {
		return nil, err
	}

	fmt.Println("txBytesJson", string(txBytesJson))

	txClient := tx.NewServiceClient(tm.grpcConn)
	grpcRes, err := txClient.BroadcastTx(
		context.TODO(),
		&tx.BroadcastTxRequest{
			Mode:    tx.BroadcastMode_BROADCAST_MODE_SYNC,
			TxBytes: txBytes,
		},
	)

	return grpcRes, err
}

func (tm *TransactionMaker) GetAccountInfo(address string) (*authtypes.BaseAccount, error) {
	res, err := authtypes.NewQueryClient(tm.grpcConn).
		Account(context.TODO(), &authtypes.QueryAccountRequest{
			Address: address,
		})
	if err != nil {
		return nil, err
	}

	acc := new(authtypes.BaseAccount)
	err = tm.codec.Unmarshal(res.Account.Value, acc)

	return acc, err
}
