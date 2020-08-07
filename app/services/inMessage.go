package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/quangdangfit/gosdk/utils/logger"
	"github.com/spf13/viper"
	"gopkg.in/mgo.v2/bson"

	"gomq/app/models"
	"gomq/app/queue"
	"gomq/app/repositories"
	"gomq/app/schema"
	"gomq/utils"
)

const (
	RequestTimeout       = 60
	DefaultMaxRetryTimes = 3
)

type inService struct {
	cons        queue.Consumer
	inMsgRepo   repositories.InMessageRepository
	routingRepo repositories.RoutingRepository
}

func NewInService(cons queue.Consumer,
	inMsgRepo repositories.InMessageRepository,
	routingRepo repositories.RoutingRepository) InService {

	r := inService{
		cons:        cons,
		inMsgRepo:   inMsgRepo,
		routingRepo: routingRepo,
	}
	return &r
}

func (i *inService) Consume() {
	msgChan := i.cons.GetMessageChannel()
	i.cons.RunConsumer(nil)

	time.Sleep(10 * time.Second)

	for index := 0; index <= i.cons.GetThreadsNumber(); index++ {
		for msg := range msgChan {
			i.handle(msg, msg.RoutingKey.Name)
			i.inMsgRepo.AddInMessage(msg)
		}
	}
}

func (i *inService) CronRetry(limit int) error {
	query := schema.InMessageQueryParam{
		Status: models.InMessageStatusWaitRetry,
	}

	messages, _ := i.inMsgRepo.GetInMessages(&query, limit)
	if messages == nil {
		logger.Info("[Retry Message] Not found any wait_retry message!")
		return nil
	}

	logger.Infof("[Retry Message] Found %d wait_retry messages!", len(*messages))
	for _, msg := range *messages {
		err := i.handle(&msg, msg.RoutingKey.Name)
		if err == nil {
			continue
		}

		msg.Attempts += 1
		if msg.Attempts >= i.getMaxRetryTimes() {
			msg.Status = models.InMessageStatusFailed
		}
		err = i.inMsgRepo.UpdateInMessage(&msg)
		if err != nil {
			logger.Errorf("Sent, failed to update status: %s, %s, %s, error: %s",
				msg.RoutingKey.Name, msg.OriginModel, msg.OriginCode, err)
		}
	}
	logger.Info("[Retry Message] Finish!")

	return nil
}

func (i *inService) CronRetryPrevious(limit int) error {
	query := schema.InMessageQueryParam{
		Status: models.InMessageStatusWaitPrevMsg,
	}
	messages, _ := i.inMsgRepo.GetInMessages(&query, limit)
	if messages == nil {
		logger.Info("[Retry Prev Message] Not found any wait_prev message!")
		return nil
	}

	logger.Infof("[Retry Prev Message] Found %d wait_prev messages!", len(*messages))
	for _, msg := range *messages {
		query := schema.InMessageQueryParam{
			RoutingGroup: msg.RoutingKey.Group,
			RoutingValue: msg.RoutingKey.Value - 1,
		}
		prevMsg, err := i.inMsgRepo.GetSingleInMessage(&query)
		if (prevMsg == nil && msg.RoutingKey.Value != 1) ||
			(prevMsg != nil && prevMsg.Status != models.InMessageStatusSuccess &&
				prevMsg.Status != models.InMessageStatusCanceled) {

			logger.Infof("[Retry Prev Message] Ignore message %s!", msg.ID)
			continue
		}

		err = i.handle(&msg, msg.RoutingKey.Name)
		if err == nil {
			continue
		}

		msg.Attempts += 1
		if msg.Attempts >= i.getMaxRetryTimes() {
			msg.Status = models.InMessageStatusFailed
		}
		err = i.inMsgRepo.UpdateInMessage(&msg)
		if err != nil {
			logger.Errorf("Sent, failed to update status: %s, %s, %s, "+
				"error: %s", msg.RoutingKey.Name, msg.OriginModel, msg.OriginCode, err)
		}
	}
	logger.Info("[Retry Prev Message] Finish!")

	return nil
}

func (i *inService) handle(message *models.InMessage, routingKey string) error {
	query := bson.M{"name": routingKey}
	inRoutingKey, err := i.routingRepo.GetRoutingKey(query)
	if err != nil {
		message.Status = models.InMessageStatusInvalid
		message.Logs = append(message.Logs, utils.ParseLog(err))
		logger.Error("Cannot find routing key ", err)
		return err
	}
	message.RoutingKey = *inRoutingKey

	prevRoutingKey, _ := i.routingRepo.GetPreviousRoutingKey(message.RoutingKey)
	if prevRoutingKey != nil {
		prevMsg, _ := i.getPreviousMessage(*message, prevRoutingKey.Name)

		if prevMsg == nil || (prevMsg.Status != models.InMessageStatusSuccess &&
			prevMsg.Status != models.InMessageStatusCanceled) {

			message.Status = models.InMessageStatusWaitPrevMsg

			logger.Warn("Set message to WAIT_PREV_MESSAGE")
			return nil
		}
	}

	res, err := i.callAPI(message)
	if err != nil {
		message.Status = models.InMessageStatusWaitRetry
		message.Logs = append(message.Logs, utils.ParseLog(err))
		return err
	}

	if res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusUnauthorized {
		message.Status = models.InMessageStatusWaitRetry
		err = errors.New(fmt.Sprintf("failed to call API %s", res.Status))
		message.Logs = append(message.Logs, utils.ParseLog(res))
		return err
	}

	if res.StatusCode != http.StatusOK {
		message.Status = models.InMessageStatusWaitRetry
		err = errors.New("failed to call API")
		message.Logs = append(message.Logs, utils.ParseLog(res))
		return err
	}

	message.Status = models.InMessageStatusSuccess
	message.Logs = append(message.Logs, utils.ParseLog(res))

	return nil
}

func (i *inService) storeMessage(message *models.InMessage) (err error) {
	return i.inMsgRepo.UpsertInMessage(message)
}

func (i *inService) callAPI(message *models.InMessage) (*http.Response, error) {
	routingKey := message.RoutingKey

	bytesPayload, _ := json.Marshal(message.Payload)
	req, _ := http.NewRequest(
		routingKey.APIMethod, routingKey.APIUrl, bytes.NewBuffer(bytesPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", "ahsfishdi"))
	req.Header.Set("x-api-key", message.APIKey)

	client := http.Client{
		Timeout: RequestTimeout * time.Second,
	}
	res, err := client.Do(req)

	if err != nil {
		logger.Errorf("Failed to send request to %s, %s", routingKey.APIUrl, err)
		return res, err
	}

	return res, nil
}

func (i *inService) getPreviousMessage(message models.InMessage, routingKey string) (*models.InMessage, error) {

	query := schema.InMessageQueryParam{
		OriginModel: message.OriginModel,
		OriginCode:  message.OriginCode,
		RoutingKey:  routingKey,
	}
	return i.inMsgRepo.GetSingleInMessage(&query)
}

func (i *inService) getMaxRetryTimes() uint {
	retryTimes := viper.GetUint("ts_rabbit.max_retry_times")

	if retryTimes <= 0 {
		retryTimes = DefaultMaxRetryTimes
	}

	return retryTimes
}
