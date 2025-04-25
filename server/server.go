package server

import (
	"context"
	"net"
	"net/http"

	"github.com/webitel/wlog"

	"github.com/kirychukyurii/notificator/config"
)

type Server struct {
	cfg      *config.HttpServer
	log      *wlog.Logger
	mux      *http.ServeMux
	server   *http.Server
	listener net.Listener
}

func New(log *wlog.Logger, cfg *config.HttpServer) (*Server, error) {
	listener, err := net.Listen("tcp", cfg.Bind)
	if err != nil {
		return nil, err
	}

	server := &Server{
		cfg:      cfg,
		log:      log,
		mux:      http.NewServeMux(),
		listener: listener,
	}

	return server, nil
}

func (s *Server) HandleFunc(pattern string, handler http.HandlerFunc) {
	s.mux.HandleFunc(s.cfg.Root+pattern, handler)
}

func (s *Server) Start() error {
	s.server = &http.Server{
		Addr:    s.cfg.Bind,
		Handler: s.mux,
	}

	s.log.Info("serve http server", wlog.String("addr", s.listener.Addr().String()))
	if err := s.server.Serve(s.listener); err != nil {
		return err
	}

	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func (s *Server) PublicURL() string {
	return s.cfg.PublicURL + s.cfg.Root
}
