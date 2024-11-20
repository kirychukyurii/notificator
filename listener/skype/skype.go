package skype

import (
	"context"
	"sync/atomic"

	"github.com/webitel/wlog"

	"github.com/kirychukyurii/notificator/config"
	"github.com/kirychukyurii/notificator/listener/skype/connection"
	"github.com/kirychukyurii/notificator/notify"
)

type Manager struct {
	cli    *connection.Connection
	listen *atomic.Bool
}

func New(cfg *config.SkypeConfig, log *wlog.Logger, queue *notify.Queue) (*Manager, error) {
	c, err := connection.NewConnection(log, cfg.Login, cfg.Password)
	if err != nil {
		return nil, err
	}

	listen := &atomic.Bool{}
	c.AddHandler(newHandler(log, queue, listen))

	return &Manager{
		cli:    c,
		listen: listen,
	}, nil
}

func (m *Manager) Listen(ctx context.Context) error {
	m.listen.Store(true)
	subscribed := make(chan bool, 1)
	errCh := make(chan error, 1)
	go func() {
		if err := m.cli.Poll(subscribed); err != nil {
			errCh <- err
		}
	}()

	for {
		select {
		case err := <-errCh:
			return err
		case ok := <-subscribed:
			if ok {
				return nil
			}
		}
	}
}

func (m *Manager) String() string {
	return ""
}

func (m *Manager) Close() error {
	m.listen.Store(false)
	return m.cli.Unsubscribe()
}
