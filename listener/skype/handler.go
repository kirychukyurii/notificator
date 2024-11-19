package skype

import (
	"github.com/webitel/wlog"

	"github.com/kirychukyurii/notificator/listener/skype/connection"
	"github.com/kirychukyurii/notificator/model"
	"github.com/kirychukyurii/notificator/notify"
)

var _ connection.TextMessageHandler = &handler{}

type handler struct {
	log   *wlog.Logger
	queue *notify.Queue
}

func newHandler(log *wlog.Logger, queue *notify.Queue) *handler {
	return &handler{
		log:   log,
		queue: queue,
	}
}

func (h *handler) HandleError(err error) {
	h.log.Error("skype handle event", wlog.Err(err))
}

func (h *handler) HandleTextMessage(message connection.Resource) {
	h.queue.Push(&model.Alert{
		Channel: "skype",
		Text:    message.Content,
		From:    message.From,
		Chat:    message.ConversationLink,
	})
}
