package stdout

import (
	"context"

	"github.com/webitel/wlog"

	"github.com/kirychukyurii/notificator/config"
	"github.com/kirychukyurii/notificator/model"
)

type Logger struct {
	name string
	log  *wlog.Logger
}

func New(name string, log *wlog.Logger) (*Logger, error) {
	return &Logger{name: name, log: log}, nil
}

func (l *Logger) Notify(ctx context.Context, onduty *config.Technical, alert ...*model.Alert) (bool, error) {
	l.log.Info("receive alerts", wlog.Any("alerts", alert), wlog.Any("on_duty", onduty))

	return false, nil
}

func (l *Logger) String() string {
	return l.name
}
