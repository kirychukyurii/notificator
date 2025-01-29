package client

import (
	"time"

	"github.com/webitel/wlog"
)

type Handler func(message *Resource)

// AddHandler adds an handler to the list of handler that receives dispatched messages.
// The provided handler must at least implement the EventHandler interface.
// Additionally, implemented handlers(TextMessageHandler, ImageMessageHandler) are optional.
// At runtime it is checked if they are implemented
// and they are called if so and needed.
func (c *Client) AddHandler(handler Handler) {
	c.handlers = append(c.handlers, handler)
}

// ClearHandlers empties the list of handlers that receive dispatched messages.
func (c *Client) ClearHandlers() {
	c.handlers = make([]Handler, 0)
}

func (c *Client) handle(message *Conversation) {
	switch message.ResourceType {
	case "NewMessage":
		t, err := time.Parse(time.RFC3339, message.Resource.ComposeTime)
		if err != nil {
			c.log.Error("handle new message", wlog.Err(err))

			return
		}

		message.Resource.Timestamp = t.Unix()
		if ok := message.Resource.GetFromMe(c.profile.username); ok {
			c.log.Debug("skip outbound message", wlog.Any("outbound", ok))

			return
		}

		for _, h := range c.handlers {
			go h(&message.Resource)
		}
	default:
		c.log.Warn("receive unspecified event type", wlog.String("type", message.ResourceType))
	}
}
