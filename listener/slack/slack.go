package slack

import (
	"context"
	"fmt"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"github.com/webitel/wlog"

	"github.com/kirychukyurii/notificator/config/listener"
)

type Manager struct {
	log  *wlog.Logger
	conn *socketmode.Client
}

func New(cfg *listener.SlackConfig, log *wlog.Logger) (*Manager, error) {
	cli := slack.New(cfg.BotToken, slack.OptionDebug(true), slack.OptionAppLevelToken(cfg.AppToken))
	conn := socketmode.New(cli, socketmode.OptionDebug(true))

	return &Manager{
		log:  log,
		conn: conn,
	}, nil
}

func (m *Manager) Listen(ctx context.Context) error {
	h := socketmode.NewSocketmodeHandler(m.conn)

	h.Handle(socketmode.EventTypeEventsAPI, middlewareEventsAPI)
	h.HandleEvents(slackevents.Message, m.receiveMessageEvent)

	return h.RunEventLoopContext(ctx)
}

func (m *Manager) String() string {
	return "slack"
}

func (m *Manager) Close() error {
	return nil
}

func (m *Manager) receiveMessageEvent(evt *socketmode.Event, client *socketmode.Client) {
	eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
	if !ok {
		return
	}

	client.Ack(*evt.Request)

	ev, ok := eventsAPIEvent.InnerEvent.Data.(*slackevents.MessageEvent)
	if !ok {
		return
	}

	m.log.Debug("received message", wlog.String("message", ev.Text), wlog.String("channel", ev.Channel))
}

func middlewareEventsAPI(evt *socketmode.Event, client *socketmode.Client) {
	fmt.Println("middlewareEventsAPI")
	eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
	if !ok {
		fmt.Printf("Ignored %+v\n", evt)
		return
	}

	fmt.Printf("Event received: %+v\n", eventsAPIEvent)

	client.Ack(*evt.Request)

	switch eventsAPIEvent.Type {
	case slackevents.CallbackEvent:
		innerEvent := eventsAPIEvent.InnerEvent
		switch ev := innerEvent.Data.(type) {
		case *slackevents.AppMentionEvent:
			fmt.Printf("We have been mentionned in %v", ev.Channel)
			_, _, err := client.Client.PostMessage(ev.Channel, slack.MsgOptionText("Yes, hello.", false))
			if err != nil {
				fmt.Printf("failed posting message: %v", err)
			}
		case *slackevents.MemberJoinedChannelEvent:
			fmt.Printf("user %q joined to channel %q", ev.User, ev.Channel)
		}
	default:
		client.Debugf("unsupported Events API event received")
	}
}
