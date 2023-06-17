package main

import (
	"errors"
	"github.com/redis/go-redis/v9"
	"html/template"
	"net/http"
)

type HTTPServer struct {
	logger Logger
	store  CodeStore
}

func NewHTTPServer(logger Logger, store CodeStore) *HTTPServer {
	return &HTTPServer{logger: logger, store: store}
}

func (h *HTTPServer) handleHomePage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./static/index.html")
}

func (h *HTTPServer) handleCodePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte("405 - Method Not Allowed"))
		return
	}
	key := r.URL.Path[3:]
	code, err := h.store.Get(r.Context(), key)
	switch {
	case errors.Is(err, redis.Nil):
		h.logger.Infow("key not found", "key", key)
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("404 - Not Found"))
		return
	case err != nil:
		h.logger.Errorw("failed to get key from redis", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("500 - Something bad happened!"))
		return
	}
	h.logger.Debugw("fetched code from store", "key", key, "code", string(code))

	t, err := template.New("code.html").ParseFiles("./templates/code.html")
	if err != nil {
		h.logger.Errorw("failed to parse template", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("500 - Something bad happened!"))
		return
	}
	err = t.Execute(w, string(code))
	if err != nil {
		h.logger.Errorw("failed to execute template", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("500 - Something bad happened!"))
		return
	}
}

func (h *HTTPServer) ListenAndServe(addr string) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/", h.handleHomePage)
	mux.HandleFunc("/c/", h.handleCodePage)

	fs := http.FileServer(http.Dir("./static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))
	return http.ListenAndServe(addr, mux)
}
