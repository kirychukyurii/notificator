package teams

import (
	"context"
	"fmt"

	"github.com/webitel/wlog"

	"github.com/kirychukyurii/notificator/config/listeners"
	"github.com/kirychukyurii/notificator/notifier"
	"github.com/kirychukyurii/notificator/server"
)

type Manager struct {
	auth *auth
}

func New(cfg *listeners.TeamsConfig, log *wlog.Logger, queue *notifier.Queue, srv *server.Server) (*Manager, error) {
	authcli, err := newAuth(context.TODO(), cfg, log, srv, queue)
	if err != nil {
		return nil, fmt.Errorf("create authentication client: %w", err)
	}

	return &Manager{auth: authcli}, nil
}

func (m Manager) Listen(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func (m Manager) String() string {
	return "teams"
}

func (m Manager) Close() error {
	//TODO implement me
	panic("implement me")
}
