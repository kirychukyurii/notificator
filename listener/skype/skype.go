package skype

import (
	"context"
	"errors"

	"github.com/webitel/wlog"

	"github.com/kirychukyurii/notificator/config/listener"
	"github.com/kirychukyurii/notificator/listener/skype/client"
	"github.com/kirychukyurii/notificator/model"
	"github.com/kirychukyurii/notificator/notifier"
)

type Manager struct {
	log   *wlog.Logger
	queue *notifier.Queue
	cli   *client.Client

	stopFunc context.CancelFunc
}

func New(cfg *listener.SkypeConfig, log *wlog.Logger, queue *notifier.Queue) (*Manager, error) {
	c, err := client.New(log, cfg.Login, cfg.Password)
	if err != nil {
		return nil, err
	}

	return &Manager{
		log:   log,
		queue: queue,
		cli:   c,
	}, nil
}

func (m *Manager) Listen(ctx context.Context) error {
	ctx, m.stopFunc = context.WithCancel(ctx)
	m.cli.AddHandler(newHandler(m.queue))
	errCh := make(chan error, 1)
	go func() {
		if err := m.cli.Poll(ctx); err != nil {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		if !errors.Is(ctx.Err(), context.Canceled) {
			return ctx.Err()
		}

		return nil
	case err := <-errCh:
		return err
	}
}

func (m *Manager) String() string {
	return ""
}

func (m *Manager) Close() error {
	m.stopFunc()
	m.cli.ClearHandlers()

	return nil
}

func newHandler(queue *notifier.Queue) client.Handler {
	return func(message *client.Resource) {
		if message.MessageType == "RichText" || message.MessageType == "Text" {
			queue.Push(&model.Alert{
				Channel: "skype",
				Text:    message.Content,
				From:    message.ImDisplayName,
				Chat:    message.ThreadTopic,
			})
		}
	}
}
