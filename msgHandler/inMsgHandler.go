package msgHandler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"gomq/dbs"
	"gomq/models"
	"gomq/utils"
	"net/http"
	"time"

	"gitlab.com/quangdangfit/gocommon/utils/logger"
)

const RequestTimeout = time.Duration(60 * time.Second)

type InMessageHandler interface {
	HandleMessage(message *models.InMessage, routingKey string) (*models.InMessage, error)
	storeMessage(message *models.InMessage) (err error)
	callAPI(message *models.InMessage) (*http.Response, error)
}

type receiver struct {
	store bool
}

func NewInMessageHandler(store bool) InMessageHandler {
	r := receiver{store: store}
	return &r
}

func (r *receiver) HandleMessage(message *models.InMessage, routingKey string) (
	*models.InMessage, error) {

	if r.store {
		defer r.storeMessage(message)
	}

	inRoutingKey, err := dbs.GetRoutingKey(routingKey)
	if err != nil {
		message.Status = dbs.InMessageStatusInvalid
		message.Logs = append(message.Logs, utils.ParseError(err))
		logger.Error("Cannot find routing key ", err)
		return message, err
	}
	message.RoutingKey = *inRoutingKey

	res, err := r.callAPI(message)
	if err != nil {
		message.Status = dbs.InMessageStatusWaitRetry
		message.Logs = append(message.Logs, utils.ParseError(err))
		return message, err

	}

	if res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusUnauthorized {
		message.Status = dbs.InMessageStatusWaitRetry
		err = errors.New(fmt.Sprintf("not found url %s", message.RoutingKey.APIUrl))
		message.Logs = append(message.Logs, utils.ParseError(res.Status))
		return message, err
	}

	if res.StatusCode != http.StatusOK {
		message.Status = dbs.InMessageStatusWaitRetry
		err = errors.New("failed to call API")
		message.Logs = append(message.Logs, utils.ParseError(res))
		return message, err
	}

	message.Status = dbs.InMessageStatusSuccess
	message.Logs = append(message.Logs, utils.ParseError(res))

	return message, err
}

func (r *receiver) storeMessage(message *models.InMessage) (err error) {
	message, _ = dbs.AddInMessage(message)
	return nil
}

func (r *receiver) callAPI(message *models.InMessage) (*http.Response, error) {
	routingKey := message.RoutingKey

	bytesPayload, _ := json.Marshal(message.Payload)
	req, _ := http.NewRequest(
		routingKey.APIMethod, routingKey.APIUrl, bytes.NewBuffer(bytesPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", "ahsfishdi"))
	req.Header.Set("x-api-key", message.APIKey)

	client := http.Client{
		Timeout: RequestTimeout,
	}
	res, err := client.Do(req)

	if err != nil {
		logger.Errorf("Failed to send request to %s, %s", routingKey.APIUrl, err)
		return res, err
	}

	return res, nil
}
