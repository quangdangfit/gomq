package api

import (
	"github.com/gin-gonic/gin"

	"github.com/jinzhu/copier"
	"github.com/quangdangfit/gosdk/utils/logger"
	"github.com/quangdangfit/gosdk/validator"

	"gomq/app/models"
	"gomq/app/schema"
	"gomq/app/services"
	"gomq/pkg/app"
)

type Sender struct {
	service services.OutService
}

func NewSender(service services.OutService) *Sender {
	return &Sender{service: service}
}

// PublishMessage godoc
// @Tags Out Messages
// @Summary publish message to amqp
// @Description api publish out message to amqp
// @Accept  json
// @Produce json
// @Param Body body schema.OutMessageBodyParam true "Body"
// @Security ApiKeyAuth
// @Success 200 {object} app.Response
// @Header 200 {string} Token "qwerty"
// @Router /api/v1/queue/messages [post]
func (s *Sender) PublishMessage(c *gin.Context) {
	var req schema.OutMessageBodyParam
	if err := c.Bind(&req); err != nil {
		logger.Error("Failed to bind body: ", err)
		app.ResError(c, err, 400)
		return
	}

	validate := validator.New()
	if err := validate.Validate(req); err != nil {
		logger.Error("Body is invalid: ", err)
		app.ResError(c, err, 400)
		return
	}

	message, err := s.parseMessage(c, req)
	if err != nil {
		logger.Error("Failed to parse out message: ", err)
		app.ResError(c, err, 400)
		return
	}

	err = s.service.Publish(c, message)
	if err != nil {
		logger.Error("Failed to publish message: ", err)
		app.ResError(c, err, 400)
		return
	}

	app.ResOK(c)
}

func (s *Sender) parseMessage(c *gin.Context, body schema.OutMessageBodyParam) (
	*models.OutMessage, error) {
	message := models.OutMessage{}
	err := copier.Copy(&message, &body)

	if err != nil {
		return &message, err
	}
	message.Status = models.OutMessageStatusWait
	message.APIKey = s.getAPIKey(c)

	return &message, nil
}

func (s *Sender) getAPIKey(c *gin.Context) string {
	return c.Request.Header.Get("X-Api-Key")
}
