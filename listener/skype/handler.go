package skype

import (
	"sync/atomic"

	"github.com/webitel/wlog"

	"github.com/kirychukyurii/notificator/listener/skype/connection"
	"github.com/kirychukyurii/notificator/model"
	"github.com/kirychukyurii/notificator/notify"
)

var _ connection.TextMessageHandler = &handler{}

type handler struct {
	log    *wlog.Logger
	queue  *notify.Queue
	listen *atomic.Bool
}

func newHandler(log *wlog.Logger, queue *notify.Queue, listen *atomic.Bool) *handler {
	return &handler{
		log:    log,
		queue:  queue,
		listen: listen,
	}
}

func (h *handler) HandleError(err error) {
	h.log.Error("skype handle event", wlog.Err(err))
}

func (h *handler) HandleTextMessage(message connection.Resource) {
	if !h.listen.Load() {
		return
	}

	h.queue.Push(&model.Alert{
		Channel: "skype",
		Text:    message.Content,
		From:    message.From,
		Chat:    message.ConversationLink,
	})
}
