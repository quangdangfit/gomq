package msgQueue

import (
	"encoding/json"
	"github.com/streadway/amqp"
	"gitlab.com/quangdangfit/gocommon/utils/logger"
	"gomq/config"
	"gomq/dbs"
	"gomq/models"
	"gomq/msgHandler"
	"gomq/utils"
)

type Publisher interface {
	MessageQueue
	Publish(message *models.OutMessage, reliable bool) error
	confirmAndHandle(message *models.OutMessage, confirms chan amqp.Confirmation) error
}

type publisher struct {
	messageQueue
	store bool
}

func NewPublisher(store bool) Publisher {
	var pub publisher

	pub.config = &models.AMQPConfig{
		AMQPUrl:      config.Config.AMQP.URL,
		ExchangeName: config.Config.AMQP.ExchangeName,
		ExchangeType: config.Config.AMQP.ExchangeType,
		QueueName:    config.Config.AMQP.QueueName,
	}
	pub.store = store
	_, err := pub.newConnection()
	if err != nil {
		logger.Error("Publisher create new connection failed!")
	}

	err = pub.declareExchange()
	if err != nil {
		logger.Error("Publisher declare exchange failed!")
	}

	return &pub
}

func (pub *publisher) Publish(message *models.OutMessage, reliable bool) (
	err error) {

	// New channel and close after publish
	pub.ensureConnection()
	channel, _ := pub.connection.Channel()
	defer channel.Close()

	// Reliable publisher confirms require confirm.select support from the connection.
	if reliable {
		if err := channel.Confirm(false); err != nil {
			logger.Errorf("Channel could not be put into confirm mode: %s", err)
			return err
		}
		confirms := channel.NotifyPublish(make(chan amqp.Confirmation, 1))
		defer pub.confirmAndHandle(message, confirms)
	}

	payload, _ := json.Marshal(message.Payload)
	headers := amqp.Table{
		"origin_code":  message.OriginCode,
		"origin_model": message.OriginModel,
		"api_key":      message.APIKey,
	}
	if err = channel.Publish(
		pub.config.ExchangeName, // publish to an exchange
		message.RoutingKey,
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			Headers:         headers,
			ContentType:     "application/json",
			ContentEncoding: "",
			Body:            payload,
			DeliveryMode:    amqp.Transient, // 1=non-persistent, 2=persistent
			Priority:        0,              // 0-9
			// a bunch of application/implementation-specific fields
		},
	); err != nil {
		message.Status = dbs.OutMessageStatusFailed
		message.Logs = append(message.Logs, utils.ParseError(err))
		logger.Error("Failed to publish message ", err)
		return err
	}

	return nil
}

func (pub *publisher) confirmAndHandle(message *models.OutMessage, confirms chan amqp.Confirmation) error {
	pub.confirmOne(message, confirms)

	outHandler := msgHandler.NewOutMessageHandler()
	_, err := outHandler.HandleMessage(message, pub.store)
	return err
}
