package models

import (
	"time"
)

const (
	CollectionInMessage = "in_message"

	InMessageStatusReceived    = "received"
	InMessageStatusSuccess     = "success"
	InMessageStatusWaitRetry   = "wait_retry"
	InMessageStatusWorking     = "working"
	InMessageStatusFailed      = "failed"
	InMessageStatusInvalid     = "invalid"
	InMessageStatusWaitPrevMsg = "wait_prev_msg"
	InMessageStatusCanceled    = "canceled"
)

type InMessage struct {
	ID          string        `json:"id,omitempty" bson:"id,omitempty"`
	RoutingKey  RoutingKey    `json:"routing_key,omitempty" bson:"routing_key,omitempty"`
	Payload     interface{}   `json:"payload,omitempty" bson:"payload,omitempty"`
	OriginCode  string        `json:"origin_code,omitempty" bson:"origin_code,omitempty"`
	OriginModel string        `json:"origin_model,omitempty" bson:"origin_model,omitempty"`
	Status      string        `json:"status,omitempty" bson:"status,omitempty"`
	Logs        []interface{} `json:"logs,omitempty" bson:"logs,omitempty"`
	Attempts    uint          `json:"attempts" bson:"attempts"`
	APIKey      string        `json:"api_key,omitempty" bson:"api_key,omitempty"`

	CreatedTime time.Time `json:"created_time" bson:"created_time"`
	UpdatedTime time.Time `json:"updated_time" bson:"updated_time"`
}