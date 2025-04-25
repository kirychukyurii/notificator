package notifier

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mymmrac/telego"
	"github.com/webitel/wlog"

	"github.com/kirychukyurii/notificator/config"
	"github.com/kirychukyurii/notificator/manager"
	"github.com/kirychukyurii/notificator/model"
)

type cache map[string]any // TODO

type Message struct {
	Channel string
	Content any
}

type Queue struct {
	log       *wlog.Logger
	notifiers []Notifier
	bot       *manager.Bot
	cache     cache

	wait       time.Duration
	items      chan *Message
	processing atomic.Bool
	mu         *sync.Mutex

	onduty *config.Technical
}

func NewQueue(log *wlog.Logger, wait time.Duration, notifiers []Notifier, bot *manager.Bot) *Queue {
	return &Queue{
		log:        log,
		notifiers:  notifiers,
		bot:        bot,
		cache:      make(cache),
		wait:       wait,
		items:      make(chan *Message),
		processing: atomic.Bool{},
		mu:         &sync.Mutex{},
	}
}

func (q *Queue) Push(v *Message) {
	q.mu.Lock()
	q.log.Debug("push alert to queue", wlog.String("channel", v.Channel))
	q.items <- v
	q.mu.Unlock()
}

func (q *Queue) WithOnDuty(onduty *config.Technical) {
	q.onduty = onduty
}

func (q *Queue) Process(ctx context.Context) {
	alerts := make([]*model.Alert, 0)
	for item := range q.items {
		switch v := item.Content.(type) {
		case *model.AuthCodeURL:
			if item.Channel == "auth_code_url" {
				ib := telego.InlineKeyboardMarkup{
					InlineKeyboard: [][]telego.InlineKeyboardButton{
						{
							{
								Text: "Login",
								URL:  v.URL,
							},
						},
					},
				}

				mp := &telego.SendMessageParams{
					Text:        "Please, login to account using your browser",
					ReplyMarkup: &ib,
				}

				message, err := q.bot.SendMessage(mp)
				if err != nil {
					q.log.Error("send message", wlog.Err(err))

					return
				}

				q.cache[v.URL] = message
			}

			if item.Channel == "resolve_auth_code_url" {
				if m, ok := q.cache[v.URL]; ok {
					mp := &telego.EditMessageTextParams{
						MessageID: m.(*telego.Message).MessageID,
						Text:      "Successfully logged in",
					}

					if err := q.bot.EditMessage(mp); err != nil {
						q.log.Error("edit message", wlog.Err(err))

					}
				}
			}

		case *model.Alert:
			if q.onduty != nil {
				alerts = append(alerts, v)
				if !q.processing.Load() {
					q.processing.Store(true)
					go func() {
						q.log.Info("process first alerts in group, waiting for other", wlog.Any("duration", q.wait))
						alerts = q.Notify(ctx, q.onduty, alerts...)

						ticker := time.NewTicker(q.wait)
						defer ticker.Stop()

						select {
						case <-ticker.C:
							q.log.Info("stop group wait, listening to new alert")
							q.processing.Store(false)
						}
					}()
				}
			}
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
