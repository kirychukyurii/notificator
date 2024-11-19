package manager

import (
	"fmt"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
	"github.com/webitel/wlog"

	"github.com/kirychukyurii/notificator/config"
)

type Bot struct {
	cfg *config.Manager
	log *wlog.Logger
	cli *telego.Bot
	bh  *th.BotHandler

	onduty chan string
}

func NewBot(cfg *config.Manager, log *wlog.Logger) (*Bot, error) {
	bot, err := telego.NewBot(cfg.BotID)
	if err != nil {
		return nil, err
	}

	opts := &telego.GetUpdatesParams{
		AllowedUpdates: []string{"callback_query"},
	}

	updates, err := bot.UpdatesViaLongPolling(opts)
	if err != nil {
		return nil, err
	}

	// Create bot handler with stop timeout
	bh, err := th.NewBotHandler(bot, updates)
	if err != nil {
		return nil, err
	}

	return &Bot{
		cfg:    cfg,
		log:    log,
		cli:    bot,
		bh:     bh,
		onduty: make(chan string),
	}, nil
}

func (b *Bot) Close() error {
	b.cli.StopLongPolling()
	b.bh.Stop()
	close(b.onduty)

	return nil
}

func (b *Bot) SendMessage(technicals []*config.Technical) error {
	var row []telego.InlineKeyboardButton
	var rows [][]telego.InlineKeyboardButton

	for _, technical := range technicals {
		row = append(row, tu.InlineKeyboardButton(technical.Name).WithCallbackData(technical.Phone))
		if len(row) == 2 {
			rows = append(rows, row)
			row = row[:0]
		}
	}

	message := tu.Message(tu.ID(b.cfg.ChatID), "Choose technical").WithReplyMarkup(tu.InlineKeyboard(rows...))
	m, err := b.cli.SendMessage(message)
	if err != nil {
		return err
	}

	b.log.Info(fmt.Sprintf("message was sent to %d, please, choose technical onduty", b.cfg.ChatID), wlog.Any("technicals", technicals))

	b.bh.Handle(b.handle(m.MessageID))
	go b.bh.Start()

	return nil
}

func (b *Bot) OnDuty() chan string {
	return b.onduty
}

func (b *Bot) handle(id int) th.Handler {
	return func(bot *telego.Bot, update telego.Update) {
		if id == update.CallbackQuery.Message.GetMessageID() {
			b.log.Info("received onduty technical", wlog.String("phone", update.CallbackQuery.Data))
			b.onduty <- update.CallbackQuery.Data

			opts := &telego.EditMessageTextParams{
				ChatID: telego.ChatID{
					ID: b.cfg.ChatID,
				},
				MessageID: update.CallbackQuery.Message.GetMessageID(),
				Text:      fmt.Sprintf("Received onduty technical: %s", update.CallbackQuery.Data),
			}

			_, err := bot.EditMessageText(opts)
			if err != nil {
				return
			}
		}
	}
}
