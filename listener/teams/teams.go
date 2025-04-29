package teams

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
	abstractions "github.com/microsoft/kiota-abstractions-go"
	"github.com/microsoft/kiota-abstractions-go/authentication"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
	msgraphmodels "github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/webitel/wlog"

	"github.com/kirychukyurii/notificator/config/listeners"
	"github.com/kirychukyurii/notificator/model"
	"github.com/kirychukyurii/notificator/notifier"
	"github.com/kirychukyurii/notificator/server"
)

type Manager struct {
	log *wlog.Logger

	auth  *auth
	queue *notifier.Queue

	cli       *msgraphsdk.GraphServiceClient
	subsID    string
	subsState string // to check that the change notification came from the subscription
	subsURL   string
}

func New(cfg *listeners.TeamsConfig, log *wlog.Logger, queue *notifier.Queue, srv *server.Server, sessionDir string) (*Manager, error) {
	ctx := context.Background()
	authcli, err := newAuth(ctx, cfg, log, srv, queue, sessionDir)
	if err != nil {
		return nil, fmt.Errorf("create authentication client: %w", err)
	}

	provider := authentication.NewBaseBearerTokenAuthenticationProvider(authcli)
	adapter, err := msgraphsdk.NewGraphRequestAdapter(provider)
	if err != nil {
		return nil, err
	}

	cli := msgraphsdk.NewGraphServiceClient(adapter)
	m := &Manager{
		log:       log,
		auth:      authcli,
		queue:     queue,
		cli:       cli,
		subsState: uuid.New().String(),
		subsURL:   srv.PublicURL() + "/subscription",
	}

	srv.HandleFunc("/subscription/{subs_state}", m.handleSubsCallback)
	return m, nil
}

func (m *Manager) Listen(ctx context.Context) error {
	if err := m.createSubscription(ctx); err != nil {
		return fmt.Errorf("create subscription: %w", err)
	}

	return nil
}

func (m *Manager) String() string {
	return "teams"
}

func (m *Manager) Close() error {
	if m.subsID != "" {
		if err := m.cli.Subscriptions().BySubscriptionId(m.subsID).Delete(context.TODO(), nil); err != nil {
			return fmt.Errorf("delete subscription: %w", err)
		}
	}

	return nil
}

func (m *Manager) createSubscription(ctx context.Context) error {
	expiration := time.Now().Add(1 * time.Hour).UTC()
	subscription := msgraphmodels.NewSubscription()
	subscription.SetChangeType(toPTR("created"))
	subscription.SetNotificationUrl(toPTR(m.subsURL + "/" + m.subsState))
	subscription.SetResource(toPTR("/me/chats/getAllMessages"))
	subscription.SetExpirationDateTime(&expiration)
	subscription.SetClientState(&m.subsState)
	// 'lifecycleNotificationUrl' is a required property for subscription creation on this resource when the 'expirationDateTime' value is set to greater than 1 hour
	// subscription.SetLifecycleNotificationUrl()
	s, err := m.cli.Subscriptions().Post(ctx, subscription, nil)
	if err != nil {
		return err
	}

	m.subsID = *s.GetId()
	go func() {
		ticker := time.NewTicker(time.Until(*s.GetExpirationDateTime()).Truncate(10 * time.Minute))
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				expiration = s.GetExpirationDateTime().Add(1 * time.Hour)
				subscription.SetExpirationDateTime(&expiration)
				s, err = m.cli.Subscriptions().BySubscriptionId(*s.GetId()).Patch(ctx, subscription, nil)
				if err != nil {
					m.log.Error("renew subscription", wlog.Err(err))
					if isInvalidSubscriptionError(err) {
						s, err = m.cli.Subscriptions().Post(ctx, subscription, nil)
						if err != nil {
							m.log.Error("create new subscription", wlog.Err(err))

							continue
						}

						m.subsID = *s.GetId()
						m.log.Info("created a new subscription", wlog.String("subscription", *s.GetId()), wlog.String("until", s.GetExpirationDateTime().String()))
					}

					continue
				}

				m.log.Info("subscription renewed", wlog.String("subscription", *s.GetId()), wlog.String("until", s.GetExpirationDateTime().String()))
			}
		}
	}()

	return nil
}

type ChangeNotification struct {
	Value []NotificationItem `json:"value"`
}

type NotificationItem struct {
	ID                             string       `json:"id"`
	SubscriptionID                 string       `json:"subscriptionId"`
	SubscriptionExpirationDateTime time.Time    `json:"subscriptionExpirationDateTime"`
	ClientState                    string       `json:"clientState"`
	ChangeType                     string       `json:"changeType"`
	Resource                       string       `json:"resource"`
	TenantID                       string       `json:"tenantId"`
	ResourceData                   ResourceData `json:"resourceData"`
}

type ResourceData struct {
	ODataType string `json:"@odata.type"`
	ODataID   string `json:"@odata.id"`
	ODataETag string `json:"@odata.etag"`
	ID        string `json:"id"`
}

func (m *Manager) handleSubsCallback(w http.ResponseWriter, r *http.Request) {
	validationToken := r.URL.Query().Get("validationToken")
	if validationToken != "" {
		m.log.Debug("received validation request", wlog.String("subscription", m.subsID))

		// URL-decode the validationToken
		decodedToken, err := url.QueryUnescape(validationToken)
		if err != nil {
			http.Error(w, "Invalid validation token", http.StatusBadRequest)

			return
		}

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(decodedToken))

		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Cannot read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var notification ChangeNotification
	if err := json.Unmarshal(body, &notification); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	for _, n := range notification.Value {
		if n.ClientState != m.subsState {
			m.log.Warn("invalid clientState received", wlog.String("clientState", n.ClientState))
			http.Error(w, "Invalid client state", http.StatusUnauthorized)
			return
		}

		m.queue.Push(&notifier.Message{
			Channel: "teams",
			Content: &model.Alert{
				Channel: "teams",
				// TODO: add other fields
			},
		})

	}

	w.WriteHeader(http.StatusAccepted)
}

func toPTR[T any](val T) *T {
	return &val
}

func isInvalidSubscriptionError(err error) bool {
	var respErr *abstractions.ApiError
	if errors.As(err, &respErr) {
		statusCode := respErr.GetStatusCode()

		return statusCode == 404 || statusCode == 410 || statusCode == 422 // Check for typical "subscription is gone" statuses
	}

	return false
}
