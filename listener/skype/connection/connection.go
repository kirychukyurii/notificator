package connection

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/webitel/wlog"
)

// Connection Golang implementation of SkPy (Skype for Web)
// See: https://skpy.t.allofti.me/background/authentication.html
type Connection struct {
	log      *wlog.Logger
	auth     *Auth
	handlers []Handler
}

// NewConnection Create a new Skype Connection.
//
// Username and password can be either a Skype username/password pair,
// or a Microsoft account email address and its associated password.
//
// If a token file path is present, it will be used if valid.
// On a successful connection, the token file will also be written to.
func NewConnection(log *wlog.Logger, username, password string) (*Connection, error) {
	auth, err := login(username, password)
	if err != nil {
		return nil, err
	}

	cli := &Connection{
		log:  log,
		auth: auth,
	}

	log.Info("logged user", wlog.String("first_name", cli.auth.profile.FirstName),
		wlog.String("last_name", cli.auth.profile.LastName), wlog.String("username", username),
		wlog.String("expires", cli.auth.session.SkypeExpires))

	return cli, nil
}

// Poll create event loop, that used to retrieve a list of events since the last poll.
// Multiple calls may be necessary to retrieve all events.
//
// If no events occur, the API will block for up to 30 seconds,
// after which an empty list is returned.
// As soon as an event is received in this time, it is returned immediately.
func (c *Connection) Poll(subscribed chan<- bool) error {
	if err := c.subscribe(); err != nil {
		return err
	}

	c.log.Info("subscribed to receive messages in conversations")
	subscribed <- true
	req := newClient(60 * time.Second)
	for {
		if c.auth.session.LocationHost == "" || c.auth.session.EndpointId == "" ||
			c.auth.session.SkypeToken == "" || c.auth.session.RegistrationExpires == "" {
			c.auth.loggedIn = false
		}

		if c.auth.loggedIn == false {
			break
		}

		pollPath := c.pollPath()
		header := map[string]string{
			"Authentication":    "skypetoken=" + c.auth.session.SkypeToken,
			"RegistrationToken": c.auth.session.RegistrationTokenStr,
			"BehaviorOverride":  "redirectAs404",
		}

		data := map[string]interface{}{
			"endpointFeatures": "Agent",
		}

		params, _ := json.Marshal(data)
		body, status, err := req.Post(pollPath, strings.NewReader(string(params)), nil, header)
		if err != nil {
			c.log.Warn("poll events", wlog.Err(err))
		}

		switch status {
		case 0:
			if err != nil {
				if strings.Index(err.Error(), "Client.Timeout exceeded while awaiting headers") < 0 &&
					strings.Index(err.Error(), "i/o timeout") < 0 &&
					strings.Index(err.Error(), "EOF") < 0 { // TODO "EOF" ?
					c.auth.loggedIn = false

					break
				}
			} else {
				c.auth.loggedIn = false

				break
			}
		case 401: // 401 - Authorization Failure. RegistrationToken and/or Cookie must be set.
			if c.auth.loggedIn {
				c.auth.loggedIn = false

				// skype token is invalid
				// use username and password login again
				_ = c.reLoginWithSubscribes()
			}
		case 404: // 404 - no endpoint created (need to refresh registration token)
			if c.auth.loggedIn {
				// need refresh registration token
				if err = c.auth.SkypeRegistrationTokenProvider(c.auth.session.SkypeToken); err != nil {
					c.auth.loggedIn = false

					// use username and password login again
					_ = c.reLoginWithSubscribes()
				}
			}
		}

		if body != "" {
			var bodyContent struct {
				EventMessages []Conversation `json:"eventMessages"`
				ErrorCode     int            `json:"errorCode"`
			}

			if err = json.Unmarshal([]byte(body), &bodyContent); err != nil {
				c.log.Error("json.Unmarshal poller body", wlog.Err(err))
			}

			// 729 - no endpoint created (need to refresh registration token)
			// 450 - subscription requested could not be found
			if bodyContent.ErrorCode == 729 || bodyContent.ErrorCode == 450 {
				c.log.Warn("poller response", wlog.Int("http_code", status), wlog.Int("code", bodyContent.ErrorCode), wlog.String("body", body))
				// err = c.SkypeRegistrationTokenProvider(c.LoginInfo.SkypeToken)
				// if err != nil {
				// 	c.log.Warn("poller SkypeRegistrationTokenProvider", wlog.Err(err))
				// 	continue
				// }
			}

			if len(bodyContent.EventMessages) > 0 {
				for _, message := range bodyContent.EventMessages {
					if message.Type == "EventMessage" {
						c.handle(message)
					}
				}
			}
		}
	}

	return nil
}

func (c *Connection) Unsubscribe() error {
	req := newClient(60 * time.Second)
	subscribePath := c.subscribePath()
	header := map[string]string{
		"Authentication":    "skypetoken=" + c.auth.session.SkypeToken,
		"RegistrationToken": c.auth.session.RegistrationTokenStr,
		"BehaviorOverride":  "redirectAs404",
	}

	_, _, err := req.Delete(subscribePath, nil, header)
	if err != nil {
		return err
	}

	return nil
}

func (c *Connection) reLoginWithSubscribes() error {
	auth, err := login(c.auth.session.Username, c.auth.session.Password)
	if err != nil {
		return err
	}

	c.auth = auth
	if err := c.subscribe(); err != nil {
		return err
	}

	return nil
}

// Subscribe will subscribe to events, that provides
// real-time information for messages sent and received in conversations,
// as well as endpoint and presence changes
func (c *Connection) subscribe() error {
	req := newClient(60 * time.Second)
	subscribePath := c.subscribePath()
	data := map[string]interface{}{
		"interestedResources": []string{
			"/v1/threads/ALL",
			"/v1/users/ME/contacts/ALL",
			"/v1/users/ME/conversations/ALL/messages",
			"/v1/users/ME/conversations/ALL/properties",
		},
		"template":    "raw",
		"channelType": "httpLongPoll",
	}

	header := map[string]string{
		"Authentication":    "skypetoken=" + c.auth.session.SkypeToken,
		"RegistrationToken": c.auth.session.RegistrationTokenStr,
		"BehaviorOverride":  "redirectAs404",
	}

	params, err := json.Marshal(data)
	if err != nil {
		return err
	}

	body, status, err := req.Post(subscribePath, strings.NewReader(string(params)), nil, header)
	if err != nil {
		return err
	}

	if status != http.StatusCreated {
		return fmt.Errorf("unable to subscribe to resources: code: %d, body: %s", status, body)
	}

	return nil
}

func (c *Connection) subscribePath() string {
	return c.auth.session.LocationHost + "/v1/users/ME/endpoints/" + c.auth.session.EndpointId + "/subscriptions"
}

func (c *Connection) pollPath() string {
	return c.auth.session.LocationHost + "/v1/users/ME/endpoints/" + c.auth.session.EndpointId + "/subscriptions/0/poll"
}
