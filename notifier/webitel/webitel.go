package webitel

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/go-openapi/strfmt"
	"github.com/google/uuid"
	"github.com/webitel/webitel-openapi-client-go/client"
	"github.com/webitel/webitel-openapi-client-go/client/communication_type_service"
	"github.com/webitel/webitel-openapi-client-go/client/member_service"
	"github.com/webitel/webitel-openapi-client-go/client/queue_service"
	"github.com/webitel/webitel-openapi-client-go/models"
	"github.com/webitel/wlog"

	"github.com/kirychukyurii/notificator/config"
	"github.com/kirychukyurii/notificator/config/notifiers"
	"github.com/kirychukyurii/notificator/model"
)

type Webitel struct {
	name string
	cfg  *notifiers.WebitelConfig
	log  *wlog.Logger
	cli  *client.WebitelAPI
}

func New(name string, cfg *notifiers.WebitelConfig, log *wlog.Logger) (*Webitel, error) {
	u, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, err
	}

	transport := &client.TransportConfig{
		Host:             u.Host,
		BasePath:         u.Path,
		Schemes:          []string{u.Scheme},
		APIKey:           cfg.Authorization.Value,
		TLSConfig:        &tls.Config{},
		NumRetries:       3,
		RetryTimeout:     2 * time.Second,
		RetryStatusCodes: []string{"420", "5xx"},
	}

	cli := client.NewHTTPClientWithConfig(strfmt.Default, transport)
	if _, err = cli.QueueService.ReadQueue(&queue_service.ReadQueueParams{ID: strconv.Itoa(cfg.QueueID)}); err != nil {
		return nil, err
	}

	if _, err = cli.CommunicationTypeService.ReadCommunicationType(&communication_type_service.ReadCommunicationTypeParams{ID: strconv.Itoa(cfg.TypeID)}); err != nil {
		return nil, err
	}

	return &Webitel{
		name: name,
		cfg:  cfg,
		log:  log,
		cli:  cli,
	}, nil
}

func (w *Webitel) Notify(ctx context.Context, technical *config.Technical, alert ...*model.Alert) (bool, error) {
	opts := member_service.NewCreateMemberParamsWithContext(ctx)
	opts.QueueID = strconv.Itoa(w.cfg.QueueID)

	variables := make(map[string]string, len(alert))
	variables["channel"] = alert[0].Channel
	for i, a := range alert {
		variables[fmt.Sprintf("alert-%d", i)] = a.String()
	}

	opts.Body = &models.EngineCreateMemberRequest{
		Name: fmt.Sprintf("%s: %s", technical.Name, uuid.Must(uuid.NewRandom()).String()),
		Communications: []*models.EngineMemberCommunicationCreateRequest{
			{
				Destination: technical.Phone,
				Type: &models.EngineLookup{
					ID: strconv.Itoa(w.cfg.TypeID),
				},
			},
		},
		Variables: variables,
	}

	if _, err := w.cli.MemberService.CreateMemberWithParams(opts); err != nil {
		return false, err
	}

	w.log.Info("create member at Webitel, wait for a call", wlog.Any("member", opts.Body))

	return false, nil
}

func (w *Webitel) String() string {
	return w.name
}
