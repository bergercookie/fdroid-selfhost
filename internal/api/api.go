// Package api serves the web UI, the JSON API and the F-Droid repository.
package api

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"fdroid-selfhost/internal/config"
	"fdroid-selfhost/internal/repo"
)

// Server wires the HTTP routes to the repository.
type Server struct {
	cfg         config.Config
	store       *repo.Store
	updater     *repo.Updater
	fingerprint string
	version     string
	web         fs.FS
}

func New(cfg config.Config, store *repo.Store, updater *repo.Updater, fingerprint, version string, web fs.FS) *Server {
	return &Server{cfg: cfg, store: store, updater: updater, fingerprint: fingerprint, version: version, web: web}
}

// Handler returns the complete HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/info", s.info)
	mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, s.updater.Status())
	})
	mux.HandleFunc("GET /api/apks", s.list)
	mux.HandleFunc("PUT /api/apks/{name}", s.requireToken(s.upload))
	mux.HandleFunc("DELETE /api/apks/{name}", s.requireToken(s.remove))
	mux.Handle("GET /repo/", noCache(http.StripPrefix("/repo/", http.FileServer(http.Dir(s.store.RepoDir())))))
	mux.Handle("GET /", http.FileServerFS(s.web))
	return mux
}

type info struct {
	Version     string `json:"version"`
	Name        string `json:"name"`
	RepoURL     string `json:"repoUrl"`
	Fingerprint string `json:"fingerprint"`
	FDroidLink  string `json:"fdroidLink"` // opens the "add repository" dialog in the F-Droid app
}

func (s *Server) info(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, info{
		Version:     s.version,
		Name:        s.cfg.RepoName,
		RepoURL:     s.cfg.RepoURL(),
		Fingerprint: s.fingerprint,
		FDroidLink:  fdroidLink(s.cfg.RepoURL(), s.fingerprint),
	})
}

// fdroidLink turns https://host/repo into fdroidrepos://host/repo?fingerprint=...
// (fdroidrepo:// for plain http), the scheme the F-Droid app registers.
func fdroidLink(repoURL, fingerprint string) string {
	u, err := url.Parse(repoURL)
	if err != nil {
		return ""
	}
	u.Scheme = "fdroidrepo" + strings.TrimPrefix(u.Scheme, "http")
	u.RawQuery = url.Values{"fingerprint": {fingerprint}}.Encode()
	return u.String()
}

func (s *Server) list(w http.ResponseWriter, _ *http.Request) {
	apks, err := s.store.List()
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, apks)
}

func (s *Server) upload(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.store.Save(name, r.Body, s.cfg.MaxUploadBytes); err != nil {
		switch {
		case errors.Is(err, repo.ErrInvalidName), errors.Is(err, repo.ErrNotAPK):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, repo.ErrTooLarge):
			writeError(w, http.StatusRequestEntityTooLarge, err.Error())
		default:
			serverError(w, err)
		}
		return
	}
	slog.Info("apk uploaded", "name", name)
	writeJSON(w, http.StatusCreated, map[string]string{"name": name})
}

func (s *Server) remove(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	switch err := s.store.Delete(name); {
	case err == nil:
		slog.Info("apk deleted", "name", name)
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, repo.ErrInvalidName):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, repo.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	default:
		serverError(w, err)
	}
}

// requireToken guards write endpoints with the shared bearer token.
func (s *Server) requireToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.UploadToken == "" {
			writeError(w, http.StatusServiceUnavailable, "uploads are disabled: set UPLOAD_TOKEN on the server")
			return
		}
		got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if subtle.ConstantTimeCompare([]byte(got), []byte(s.cfg.UploadToken)) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="fdroid-selfhost"`)
			writeError(w, http.StatusUnauthorized, "missing or wrong token")
			return
		}
		next(w, r)
	}
}

func noCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("writing response", "err", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func serverError(w http.ResponseWriter, err error) {
	slog.Error("request failed", "err", err)
	writeError(w, http.StatusInternalServerError, "internal error")
}
