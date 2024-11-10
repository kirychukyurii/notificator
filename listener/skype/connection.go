package skype

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Connection struct {
	auth     *Auth
	handlers []Handler
}

func NewConnection(username, password string) (*Connection, error) {
	auth, err := Login(username, password)
	if err != nil {
		return nil, err
	}

	cli := &Connection{
		auth: auth,
	}

	return cli, nil
}

func (c *Connection) Poll() error {
	if err := c.subscribe(); err != nil {
		return err
	}

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
		case 401:
			if c.auth.loggedIn {
				c.auth.loggedIn = false

				// skype token is invalid
				// use username and password login again
				_ = c.reLoginWithSubscribes()
			}
		case 404:
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
				fmt.Println("json.Unmarshal poller body err: ", err)
			}

			if bodyContent.ErrorCode == 729 || bodyContent.ErrorCode == 450 {
				fmt.Println("poller bodyContent.ErrorCode: ", bodyContent.ErrorCode)
				// err = c.SkypeRegistrationTokenProvider(c.LoginInfo.SkypeToken)
				if err != nil {
					fmt.Println("poller SkypeRegistrationTokenProvider: ", err)
					continue
				}
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

func (c *Connection) reLoginWithSubscribes() error {
	auth, err := Login(c.auth.session.Username, c.auth.session.Password)
	if err != nil {
		return err
	}

	c.auth = auth
	if err := c.subscribe(); err != nil {
		return err
	}

	return nil
}

// Subscribe will subscribe to events.
// Events provide real-time information for messages sent and received in conversations,
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

	params, _ := json.Marshal(data)
	_, _, err := req.Post(subscribePath, strings.NewReader(string(params)), nil, header)
	if err != nil {
		return err
	}

	return nil
}

func (c *Connection) subscribePath() string {
	return c.auth.session.LocationHost + "/v1/users/ME/endpoints/" + c.auth.session.EndpointId + "/subscriptions"
}

func (c *Connection) pollPath() string {
	return c.auth.session.LocationHost + "/v1/users/ME/endpoints/" + c.auth.session.EndpointId + "/subscriptions/0/poll"
}
