package webhook

import (
	"fmt"
	"net/http"

	"github.com/kirychukyurii/notificator/server"
)

type listenerFunc func(r *http.Request) error

type listener struct {
	token string
	f     listenerFunc
}

type Handler struct {
	listeners map[string]*listener
}

func NewHandler(srv *server.Server) *Handler {
	h := &Handler{
		listeners: make(map[string]*listener),
	}

	srv.HandleFunc("/{name}/{token}", h.handler)
	
	return h
}

func (s *Handler) RegisterListener(name, token string, f listenerFunc) error {
	if _, ok := s.listeners[name]; ok {
		return fmt.Errorf("listener with name %s already exists", name)
	}

	s.listeners[name] = &listener{token, f}

	return nil
}

func (s *Handler) DeregisterListener(name string) {
	if _, ok := s.listeners[name]; ok {
		delete(s.listeners, name)
	}
}

func (s *Handler) ExistsListener(name string) bool {
	_, ok := s.listeners[name]

	return ok
}

func (s *Handler) handler(w http.ResponseWriter, r *http.Request) {
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
