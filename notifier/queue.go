package notifier

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/webitel/wlog"

	"github.com/kirychukyurii/notificator/config"
	"github.com/kirychukyurii/notificator/model"
)

type Queue struct {
	log       *wlog.Logger
	notifiers []Notifier

	wait       time.Duration
	items      chan *model.Alert
	processing atomic.Bool
	mu         *sync.Mutex
}

func NewQueue(log *wlog.Logger, wait time.Duration, notifiers []Notifier) *Queue {
	return &Queue{
		log:        log,
		notifiers:  notifiers,
		wait:       wait,
		items:      make(chan *model.Alert),
		processing: atomic.Bool{},
		mu:         &sync.Mutex{},
	}
}

func (q *Queue) Push(v *model.Alert) {
	q.mu.Lock()
	q.log.Debug("push alert to queue", wlog.String("channel", v.Channel), wlog.String("text", v.Text), wlog.String("from", v.From))
	q.items <- v
	q.mu.Unlock()
}

func (q *Queue) Process(ctx context.Context, onduty *config.Technical) {
	items := make([]*model.Alert, 0)
	for item := range q.items {
		items = append(items, item)
		if !q.processing.Load() {
			q.processing.Store(true)
			go func() {
				q.log.Info("process first alerts in group, waiting for other", wlog.Any("duration", q.wait))
				items = q.Notify(ctx, onduty, item)

				defer q.log.Info("flush queue of sent alerts.. listening to new alerts")

				defer q.processing.Store(false)

				ticker := time.NewTicker(q.wait)
				defer ticker.Stop()

				select {
				case <-ticker.C:
					if len(items) > 0 {
						items = q.Notify(ctx, onduty, items...)
					}
				}
			}()
		}
	}
}

func (q *Queue) Stop() {
	close(q.items)
}

func (q *Queue) Notify(ctx context.Context, onduty *config.Technical, items ...*model.Alert) []*model.Alert {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, notifier := range q.notifiers {
		ok, err := notifier.Notify(ctx, onduty, items...)
		if err != nil {
			q.log.Error("send notify message", wlog.Err(err), wlog.Any("retry", ok))
		}
	}

	return items[:0]
}
