package main

import (
	"gomq/dbs"
	"gomq/queue"
	"gomq/repositories"

	"gitlab.com/quangdangfit/gocommon/utils/logger"
	"gopkg.in/mgo.v2/bson"
)

const ResendOutMessageLimit = 100

func main() {
	repo := repositories.NewOutMessageRepo()

	query := bson.M{"status": dbs.OutMessageStatusWait}
	messages, _ := repo.GetOutMessages(query, ResendOutMessageLimit)
	if messages == nil {
		logger.Info("[Resend Message] Not found any wait message!")
		return
	}

	pub := queue.NewPublisher()

	logger.Infof("[Resend Message] Found %d wait messages!", len(*messages))
	for _, msg := range *messages {
		pub.Publish(&msg, true)
	}
	logger.Info("[Resend Message] Finish!")
}
