package server

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/webitel/wlog"

	"github.com/kirychukyurii/notificator/config"
)

type listenerFunc func(r *http.Request) error

type listener struct {
	token string
	f     listenerFunc
}

type Server struct {
	log       *wlog.Logger
	server    *http.Server
	l         net.Listener
	listeners map[string]*listener
}

func New(log *wlog.Logger, cfg *config.HttpServer) (*Server, error) {
	l, err := net.Listen("tcp", cfg.Bind)
	if err != nil {
		return nil, err
	}

	server := &Server{
		log:       log,
		l:         l,
		listeners: make(map[string]*listener),
	}

	mux := http.NewServeMux()
	mux.HandleFunc(cfg.Root+"/{name}/{token}", server.handler)
	server.server = &http.Server{
		Addr:    cfg.Bind,
		Handler: mux,
	}

	return server, nil
}

func (s *Server) Start() error {
	s.log.Info("serve http server", wlog.String("addr", s.l.Addr().String()))
	if err := s.server.Serve(s.l); err != nil {
		return err
	}

	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func (s *Server) RegisterListener(name, token string, f listenerFunc) error {
	if _, ok := s.listeners[name]; ok {
		return fmt.Errorf("listener with name %s already exists", name)
	}

	s.listeners[name] = &listener{token, f}

	return nil
}

func (s *Server) DeregisterListener(name string) {
	if _, ok := s.listeners[name]; ok {
		delete(s.listeners, name)
	}
}

func (s *Server) ExistsListener(name string) bool {
	_, ok := s.listeners[name]

	return ok
}

func (s *Server) handler(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("webhook name required"))

		return
	}

	lis, ok := s.listeners[name]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("webhook not found"))

		return
	}

	token := r.PathValue("token")
	if token == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("webhook token required"))

		return
	}

	if lis.token != token {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("webhook token invalid"))

		return
	}

	if err := lis.f(r); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("process webhook func: " + err.Error()))

		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("process webhook func: " + name))
}
