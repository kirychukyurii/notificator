package connection

import (
	"fmt"
	"time"
)

type Handler interface {
	HandleError(err error)
}

type JsonMessageHandler interface {
	Handler
	HandleJsonMessage(message Resource)
}

// TextMessageHandler handles messagetype: RichText
type TextMessageHandler interface {
	Handler
	HandleTextMessage(message Resource)
}

// ImageMessageHandler handles messagetype: RichText/UriObject
type ImageMessageHandler interface {
	Handler
	HandleImageMessage(message Resource)
}

// ContactMessageHandler handles messagetype: RichText/UriObject
type ContactMessageHandler interface {
	Handler
	HandleContactMessage(message Resource)
}

// LocationMessageHandler handles messagetype: RichText/UriObject
type LocationMessageHandler interface {
	Handler
	HandleLocationMessage(message Resource)
}

// VideoMessageHandler messagetype:
type VideoMessageHandler interface {
	Handler
	HandleVideoMessage()
}

// messagetype:
type AudioMessageHandler interface {
	Handler
	HandleAudioMessage()
}

// EndpointPresenceHandler handles event when a user connects to Skype with a new endpoint
type EndpointPresenceHandler interface {
	Handler
	HandleEndpointPresence()
}

// UserPresenceHandler handles event when a user's availability has changed
type UserPresenceHandler interface {
	Handler
	HandlePresence(message Resource)
}

// UserTypingHandler handles event when a user's availability has changed
type UserTypingHandler interface {
	Handler
	HandleTypingStatus(message Resource)
}

// AddHandler adds an handler to the list of handler that receives dispatched messages.
// The provided handler must at least implement the Handler interface. Additionally, implemented
// handlers(TextMessageHandler, ImageMessageHandler) are optional. At runtime it is checked if they are implemented
// and they are called if so and needed.
func (c *Connection) AddHandler(handler Handler) {
	c.handlers = append(c.handlers, handler)
}

// RemoveHandler removes a handler from the list of handlers that receive dispatched messages.
func (c *Connection) RemoveHandler(handler Handler) bool {
	i := -1
	for k, v := range c.handlers {
		if v == handler {
			i = k

			break
		}
	}

	if i > -1 {
		c.handlers = append(c.handlers[:i], c.handlers[i+1:]...)

		return true
	}

	return false
}

// RemoveHandlers empties the list of handlers that receive dispatched messages.
func (c *Connection) RemoveHandlers() {
	c.handlers = make([]Handler, 0)
}

func (c *Connection) handle(message Conversation) {
	c.handleWithCustomHandlers(message, c.handlers)
}

func (c *Connection) shouldCallSynchronously(handler Handler) bool {
	return false
}

type TextMessage struct {
	Resource
}

type ChatUpdateHandler interface {
	Handler
	HandleChatUpdate(message Resource)
}

func (c *Connection) handleWithCustomHandlers(message Conversation, handlers []Handler) {
	if message.ResourceType == "NewMessage" {
		// resource := Resource{}
		// resource, ok := (message.Resource).(Resource)
		// if !ok {
		//	fmt.Println("handleWithCustomHandlers: not resource type")
		//	return
		// }
		// _ = json.Unmarshal([]byte(message.Resource), &resource)
		// ConversationLinkArr := strings.Split(message.Resource.ConversationLink, "/conversations/")
		t, _ := time.Parse(time.RFC3339, message.Resource.ComposeTime)
		message.Resource.Timestamp = t.Unix()
		message.Resource.GetFromMe(c)
		if message.Resource.MessageType == "RichText" || message.Resource.MessageType == "Text" {
			for _, h := range handlers {
				if x, ok := h.(TextMessageHandler); ok {
					if c.shouldCallSynchronously(h) {
						x.HandleTextMessage(message.Resource)
					} else {
						go x.HandleTextMessage(message.Resource)
					}
				}
			}
		} else if message.Resource.MessageType == "RichText/UriObject" {
			for _, h := range handlers {
				if x, ok := h.(ImageMessageHandler); ok {
					if c.shouldCallSynchronously(h) {
						x.HandleImageMessage(message.Resource)
					} else {
						go x.HandleImageMessage(message.Resource)
					}
				}
			}
		} else if message.Resource.MessageType == "RichText/Contacts" {
			for _, h := range handlers {
				if x, ok := h.(ContactMessageHandler); ok {
					if c.shouldCallSynchronously(h) {
						x.HandleContactMessage(message.Resource)
					} else {
						go x.HandleContactMessage(message.Resource)
					}
				}
			}
		} else if message.Resource.MessageType == "RichText/Location" {
			for _, h := range handlers {
				if x, ok := h.(LocationMessageHandler); ok {
					if c.shouldCallSynchronously(h) {
						x.HandleLocationMessage(message.Resource)
					} else {
						go x.HandleLocationMessage(message.Resource)
					}
				}
			}
		} else if message.Resource.MessageType == "RichText/Media_GenericFile" {
			for _, h := range handlers {
				if x, ok := h.(ImageMessageHandler); ok {
					if c.shouldCallSynchronously(h) {
						x.HandleImageMessage(message.Resource)
					} else {
						go x.HandleImageMessage(message.Resource)
					}
				}
			}
		} else if message.Resource.MessageType == "RichText/Media_Album" {
			for _, h := range handlers {
				if x, ok := h.(ChatUpdateHandler); ok {
					if c.shouldCallSynchronously(h) {
						x.HandleChatUpdate(message.Resource)
					} else {
						go x.HandleChatUpdate(message.Resource)
					}
				}
			}
		} else if message.Resource.MessageType == "RichText/Media_Video" {
			for _, h := range handlers {
				if x, ok := h.(ImageMessageHandler); ok {
					if c.shouldCallSynchronously(h) {
						x.HandleImageMessage(message.Resource)
					} else {
						go x.HandleImageMessage(message.Resource)
					}
				}
			}
		} else if message.Resource.MessageType == "RichText/Media_AudioMsg" {
			for _, h := range handlers {
				if x, ok := h.(ImageMessageHandler); ok {
					if c.shouldCallSynchronously(h) {
						x.HandleImageMessage(message.Resource)
					} else {
						go x.HandleImageMessage(message.Resource)
					}
				}
			}
		} else if message.Resource.MessageType == "ThreadActivity/TopicUpdate" {
			for _, h := range handlers {
				if x, ok := h.(ChatUpdateHandler); ok {
					if c.shouldCallSynchronously(h) {
						x.HandleChatUpdate(message.Resource)
					} else {
						go x.HandleChatUpdate(message.Resource)
					}
				}
			}
		} else if message.Resource.MessageType == "ThreadActivity/PictureUpdate" {
			for _, h := range handlers {
				if x, ok := h.(ChatUpdateHandler); ok {
					if c.shouldCallSynchronously(h) {
						x.HandleChatUpdate(message.Resource)
					} else {
						go x.HandleChatUpdate(message.Resource)
					}
				}
			}
		} else if message.Resource.MessageType == "Control/Typing" {
			for _, h := range handlers {
				if x, ok := h.(UserTypingHandler); ok {
					if c.shouldCallSynchronously(h) {
						x.HandleTypingStatus(message.Resource)
					} else {
						go x.HandleTypingStatus(message.Resource)
					}
				}
			}
		} else if message.Resource.MessageType == "Control/ClearTyping" {
			for _, h := range handlers {
				if x, ok := h.(UserTypingHandler); ok {
					if c.shouldCallSynchronously(h) {
						x.HandleTypingStatus(message.Resource)
					} else {
						go x.HandleTypingStatus(message.Resource)
					}
				}
			}
		} else if message.Resource.MessageType == "ThreadActivity/AddMember" {
			for _, h := range handlers {
				if x, ok := h.(ChatUpdateHandler); ok {
					if c.shouldCallSynchronously(h) {
						x.HandleChatUpdate(message.Resource)
					} else {
						go x.HandleChatUpdate(message.Resource)
					}
				}
			}
		} else if message.Resource.MessageType == "ThreadActivity/DeleteMember" {
			for _, h := range handlers {
				if x, ok := h.(ChatUpdateHandler); ok {
					if c.shouldCallSynchronously(h) {
						x.HandleChatUpdate(message.Resource)
					} else {
						go x.HandleChatUpdate(message.Resource)
					}
				}
			}
		} else {
			fmt.Println()
			fmt.Printf("unknown message type0: %+v", message)
			fmt.Println()
		}
	}
}
