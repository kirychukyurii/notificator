package skype

import (
	"context"

	"github.com/webitel/wlog"

	"github.com/kirychukyurii/notificator/config"
)

type Manager struct {
	cli *Connection
}

func New(cfg *config.SkypeConfig, log *wlog.Logger) (*Manager, error) {
	c, err := NewConnection(cfg.Login, cfg.Password)
	if err != nil {
		return nil, err
	}

	return &Manager{
		cli: c,
	}, nil
}

func (m *Manager) Listen(ctx context.Context) error {
	// TODO implement me
	panic("implement me")
}

func (m *Manager) String() string {
	// TODO implement me
	panic("implement me")
}

func (m *Manager) Close() error {
	// TODO implement me
	panic("implement me")
}
