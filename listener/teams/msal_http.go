package teams

import (
	"net/http"
)

func (a *auth) handleAuthCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	a.code <- code

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("received code"))
}
