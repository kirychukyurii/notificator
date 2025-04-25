package webhook

import (
	"context"
	"fmt"
	"net/http"

	"github.com/webitel/wlog"

	"github.com/kirychukyurii/notificator/config/listeners"
	"github.com/kirychukyurii/notificator/model"
	"github.com/kirychukyurii/notificator/notifier"
)

type Webhook struct {
	cfg *listeners.WebhookConfig
	log *wlog.Logger

	handler *Handler
	queue   *notifier.Queue
}

func New(cfg *listeners.WebhookConfig, log *wlog.Logger, queue *notifier.Queue, handler *Handler) (*Webhook, error) {
	if ok := handler.ExistsListener(cfg.Name); ok {
		return nil, fmt.Errorf("webhook %s already exists", cfg.Name)
	}

	return &Webhook{
		cfg:     cfg,
		log:     log,
		handler: handler,
		queue:   queue,
	}, nil
}

func (w *Webhook) Listen(ctx context.Context) error {
	if err := w.handler.RegisterListener(w.cfg.Name, w.cfg.Token, w.handlerFunc); err != nil {
		return err
	}

	w.log.Info("start listening")

	return nil
}

func (w *Webhook) String() string {
	return "webhook"
}

func (w *Webhook) Close() error {
	w.handler.DeregisterListener(w.cfg.Name)

	return nil
}

func (w *Webhook) handlerFunc(r *http.Request) error {
	alert := &model.Alert{
		Channel: w.cfg.Name,
		Text:    r.URL.Query().Get(w.cfg.ResponseMap.Message),
		From:    r.URL.Query().Get(w.cfg.ResponseMap.From),
		Chat:    r.URL.Query().Get(w.cfg.ResponseMap.Chat),
	}

	w.queue.Push(alert)

	return nil
}
