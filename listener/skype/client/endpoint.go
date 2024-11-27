package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/webitel/wlog"
)

const ApiMsgshost = "https://client-s.gateway.messenger.live.com"

// endpoint represents a single point of presence within Skype.
// Typically, a user with multiple devices would have one
// endpoint per device (desktop, laptop, mobile and so on).
// Endpoints are time-sensitive - they lapse after a short
// time unless kept alive (by Ping or otherwise).
type endpoint struct {
	log *wlog.Logger
	cli *httpClient

	id              string
	msgsHost        string
	regToken        string
	regTokenProps   string
	regTokenExpires string

	skypeToken string

	subscribed *atomic.Bool
}

func newEndpoint(log *wlog.Logger, cli *httpClient, skypeToken string) (*endpoint, error) {
	e := &endpoint{
		log:        log,
		cli:        cli,
		msgsHost:   ApiMsgshost,
		skypeToken: skypeToken,
		subscribed: &atomic.Bool{},
	}

	if skypeToken == "" {
		return nil, fmt.Errorf("skype token not exist")
	}

	if err := e.registrationToken(); err != nil {
		return nil, err
	}

	if err := e.configure(); err != nil {
		return nil, err
	}

	if err := e.Subscribe(); err != nil {
		return nil, err
	}

	go func() {
		if err := e.regTokenWatcher(context.TODO(), skypeToken); err != nil {
			log.Error("registration token watcher", wlog.Err(err))
		}
	}()

	go func() {
		if err := e.Ping(context.TODO(), 45*time.Second, 120); err != nil {
			log.Error("ping endpoint", wlog.Err(err))
		}
	}()

	return e, nil
}

// configure this endpoint to allow setting presence.
func (e *endpoint) configure() error {
	body := map[string]any{
		"id":          "messagingService",
		"type":        "EndpointPresenceDoc",
		"selfLink":    "uri",
		"privateInfo": map[string]any{"epname": "skype"},
		"publicInfo": map[string]any{
			"capabilities":     "",
			"type":             1,
			"skypeNameVersion": "skype.com",
			"nodeInfo":         "xx",
			"version":          "908/1.30.0.128"},
	}

	data, err := json.Marshal(body)
	if err != nil {
		return err
	}

	header := map[string]string{
		"registrationToken": e.regTokenProps,
		"Authentication":    e.skypeToken,
	}

	resp, err := e.cli.Request(http.MethodPut, fmt.Sprintf("%s/v1/users/ME/endpoints/%s/presenceDocs/messagingService", e.msgsHost, e.id), strings.NewReader(string(data)), nil, header)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	return nil
}

func (e *endpoint) ValidateRegToken() bool {
	if e.regToken == "" {
		return false
	}

	exp, err := strconv.Atoi(e.regTokenExpires)
	if err != nil {
		return false
	}

	now := time.Now()
	expires := now.Add(time.Duration(exp) * time.Second)
	if now.After(expires) {
		return false
	}

	return true
}

// Subscribe to contact and conversation events.
func (e *endpoint) Subscribe() error {
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
		"registrationToken": e.regTokenProps,
		"Authentication":    e.skypeToken,
	}

	params, err := json.Marshal(data)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("%s/v1/users/ME/endpoints/%s/subscriptions", e.msgsHost, e.id)
	body, status, err := e.cli.Post(path, strings.NewReader(string(params)), nil, header)
	if err != nil {
		return err
	}

	skypeErr := map[string]any{}
	if err := json.Unmarshal([]byte(body), &skypeErr); err != nil {
		return err
	}

	if status != http.StatusCreated {
		return fmt.Errorf("unable to subscribe to resources: path=%s code=%d, body=%s", path, status, body)
	}

	e.subscribed.Store(true)
	e.log.Debug("subscribe to contact and conversation events", wlog.String("msgs_host", e.msgsHost), wlog.String("id", e.id))

	return nil
}

// Unsubscribe delete subscriptions on contact and conversation events.
func (e *endpoint) Unsubscribe() error {
	header := map[string]string{
		"registrationToken": e.regTokenProps,
		"Authentication":    e.skypeToken,
	}

	_, _, err := e.cli.Delete(fmt.Sprintf("%s/v1/users/ME/endpoints/%s/subscriptions", e.msgsHost, e.id), nil, header)
	if err != nil {
		return err
	}

	e.subscribed.Store(false)

	e.log.Debug("delete subscriptions on contact and conversation events", wlog.String("msgs_host", e.msgsHost), wlog.String("id", e.id))

	return nil
}

// Events retrieved a list of events since the last poll.
// Multiple calls may be needed to retrieve all events.
// If no events occur, the API will block for up to 30 seconds,
// after which an empty list is returned.
// If any event occurs whilst blocked, it is returned immediately.
func (e *endpoint) Events() ([]*Conversation, error) {
	if !e.subscribed.Load() {
		return nil, fmt.Errorf("please subscribe to resources first")
	}

	header := map[string]string{
		"registrationToken": e.regTokenProps,
		"Authentication":    e.skypeToken,
		"BehaviorOverride":  "redirectAs404",
	}

	data := map[string]interface{}{
		"endpointFeatures": "Agent",
	}

	params, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	body, _, err := e.cli.Post(fmt.Sprintf("%s/v1/users/ME/endpoints/%s/subscriptions/0/poll", e.msgsHost, e.id), strings.NewReader(string(params)), nil, header)
	if err != nil {
		var netErr net.Error
		ok := errors.As(err, &netErr)
		if ok {
			if netErr.Timeout() {
				e.log.Debug("no events received", wlog.Err(netErr))
			}
		} else {
			return nil, err
		}
	}

	if body == "" {
		return nil, fmt.Errorf("poller body is empty")
	}

	var bodyContent struct {
		EventMessages []*Conversation `json:"eventMessages"`
		ErrorCode     int             `json:"errorCode"`
	}

	if err = json.Unmarshal([]byte(body), &bodyContent); err != nil {
		return nil, fmt.Errorf("unmarshal poller json body: %w", err)
	}

	if bodyContent.ErrorCode == 729 {
		return nil, fmt.Errorf("no endpoint created (need to refresh registration token)")
	}

	if bodyContent.ErrorCode == 450 {
		return nil, fmt.Errorf("subscription requested could not be found")
	}

	e.log.Debug("retrieve a list of events since the last poll", wlog.Int("size", len(bodyContent.EventMessages)), wlog.Any("messages", bodyContent.EventMessages))

	return bodyContent.EventMessages, nil
}

// Ping sends a keep-alive request for the endpoint.
// Endpoints must be kept alive by regularly pinging them.
// Skype for Web does this roughly every 45 seconds, sending a timeout value of 12.
// Blocked until context is done or error received.
func (e *endpoint) Ping(ctx context.Context, interval time.Duration, timeout int) error {
	errCh := make(chan error, 1)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ticker.C:
				if err := e.ping(timeout); err != nil {
					errCh <- err
				}
			}
		}
	}()

	e.log.Debug("create ping endpoint watcher for sending keep-alive request", wlog.String("msgs_host", e.msgsHost), wlog.String("id", e.id), wlog.String("timeout", strconv.Itoa(timeout)))

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

func (e *endpoint) ping(timeout int) error {
	body := map[string]any{
		"timeout": timeout,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return err
	}

	header := map[string]string{
		"Registrationtoken": e.regTokenProps,
		"Authentication":    e.skypeToken,
	}

	respBody, status, err := e.cli.Post(fmt.Sprintf("%s/v1/users/ME/endpoints/%s/active", e.msgsHost, e.id), strings.NewReader(string(data)), nil, header)
	if err != nil {
		return err
	}

	if status != http.StatusCreated {
		return fmt.Errorf("endpoint is not alive: %s", respBody)
	}

	e.log.Debug("ping active endpoint", wlog.String("body", respBody), wlog.Int("code", status), wlog.String("msgs_host", e.msgsHost), wlog.String("id", e.id), wlog.String("timeout", strconv.Itoa(timeout)))

	return nil
}

// regTokenWatcher watch registration token expires and renews
// token with its endpoint. Also subscribe to events on new endpoint.
// Blocked until context is done or error returned.
func (e *endpoint) regTokenWatcher(ctx context.Context, skypeToken string) error {
	errCh := make(chan error, 1)
	exp, err := strconv.Atoi(e.regTokenExpires)
	if err != nil {
		return err
	}

	ticker := time.NewTicker(time.Duration(exp-100) * time.Second)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ticker.C:
				e.skypeToken = skypeToken
				if err := e.registrationToken(); err != nil {
					errCh <- err
				}

				if err := e.Subscribe(); err != nil {
					errCh <- err
				}
			}
		}
	}()

	e.log.Debug("registration token watcher started", wlog.Any("interval", time.Duration(exp-100)*time.Second))

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}
