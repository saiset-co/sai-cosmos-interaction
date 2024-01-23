package internal

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/saiset-co/saiCosmosInteraction/internal/model"
	"github.com/saiset-co/saiCosmosInteraction/utils"
	"github.com/saiset-co/saiService"
)

func (is *InternalService) NewHandler() saiService.Handler {
	return saiService.Handler{
		"make_tx": saiService.HandlerElement{
			Name:        "make_tx",
			Description: "Make new transaction with type /cosmos.bank.v1beta1.MsgSend",
			Function:    is.makeTx,
		},
	}
}

func (is *InternalService) makeTx(data, _ interface{}) (interface{}, int, error) {
	var internalError = fmt.Errorf("something went wrong")

	body, err := is.validateBody(data)
	if err != nil {
		return "", http.StatusBadRequest, err
	}

	fileBytes, err := os.ReadFile(body.From)
	if err != nil {
		log.Println(body.From, err)
		return "", http.StatusInternalServerError, fmt.Errorf("don't have private key for %s", body.From)
	}

	txMaker, err := NewTransactionMaker(
		body.NodeAddress,
		body.ChainID,
		body.From,
		body.To,
		body.Passphrase,
		fileBytes,
	)

	if err != nil {
		log.Println(body.From, err)
		return "", http.StatusInternalServerError, internalError
	}

	err = txMaker.BuildTx(uint64(body.GasLimit), body.Amount, body.FeeAmount, body.Memo)
	if err != nil {
		log.Println(body.From, err)
		return "", http.StatusInternalServerError, internalError
	}

	err = txMaker.SignTx()
	if err != nil {
		log.Println(body.From, err)
		return "", http.StatusInternalServerError, internalError
	}

	txHash, err := txMaker.BroadcastTx()
	if err != nil {
		log.Println(body.From, err)
		return "", http.StatusInternalServerError, internalError
	}

	return txHash, http.StatusOK, nil
}

func (is *InternalService) validateBody(data interface{}) (model.MakeTxRequestBody, error) {
	body := model.MakeTxRequestBody{}
	dataMap, ok := data.(map[string]interface{})
	if !ok {
		return body, fmt.Errorf("wrong request body")
	}

	body.NodeAddress, ok = dataMap["node_address"].(string)
	if !ok {
		return body, fmt.Errorf("node_address field not string")
	}

	body.From, ok = dataMap["from"].(string)
	if !ok {
		return body, fmt.Errorf("from field not string")
	}

	body.To, ok = dataMap["to"].(string)
	if !ok {
		return body, fmt.Errorf("to field not string")
	}

	body.ChainID, ok = dataMap["chain_id"].(string)
	if !ok {
		return body, fmt.Errorf("chain_id field not string")
	}

	body.Passphrase, ok = dataMap["passphrase"].(string)
	if !ok {
		return body, fmt.Errorf("passphrase field not string")
	}

	var err error
	body.Amount, err = utils.IfaceToInt64(dataMap["amount"])
	if err != nil {
		return body, fmt.Errorf("amount field not int64")
	}

	body.GasLimit, err = utils.IfaceToInt64(dataMap["gas_limit"])
	if err != nil {
		return body, fmt.Errorf("gas_limit field not int64")
	}

	body.FeeAmount, err = utils.IfaceToInt64(dataMap["fee_amount"])
	if err != nil {
		return body, fmt.Errorf("fee_amount field not int64")
	}

	return body, nil
}
