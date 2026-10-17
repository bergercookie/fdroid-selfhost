// Package config loads the server configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Config is the complete server configuration. See docs/server.md.
type Config struct {
	ListenAddr      string
	DataDir         string
	PublicURL       string // how clients reach this server, without trailing slash
	RepoName        string
	RepoDescription string
	UploadToken     string // empty disables uploads and deletes
	KeystorePass    string // optional; generated and stored in DataDir when empty
	FdroidBin       string
	KeytoolBin      string
	ApksignerPath   string
	PollInterval    time.Duration
	MaxUploadBytes  int64
}

// RepoURL is the address F-Droid clients add as a repository.
func (c Config) RepoURL() string { return c.PublicURL + "/repo" }

// Load reads the configuration using getenv (os.Getenv in production).
func Load(getenv func(string) string) (Config, error) {
	get := func(key, fallback string) string {
		if v := getenv(key); v != "" {
			return v
		}
		return fallback
	}

	poll, err := time.ParseDuration(get("POLL_INTERVAL", "5s"))
	if err != nil {
		return Config{}, fmt.Errorf("POLL_INTERVAL must be a duration such as 5s: %w", err)
	}
	if poll <= 0 {
		return Config{}, errors.New("POLL_INTERVAL must be positive")
	}
	maxMB, err := strconv.ParseInt(get("MAX_UPLOAD_MB", "500"), 10, 64)
	if err != nil || maxMB <= 0 {
		return Config{}, errors.New("MAX_UPLOAD_MB must be a positive integer")
	}

	cfg := Config{
		ListenAddr:      get("LISTEN_ADDR", ":8080"),
		DataDir:         get("DATA_DIR", "/data"),
		PublicURL:       strings.TrimRight(get("PUBLIC_URL", "http://localhost:8080"), "/"),
		RepoName:        get("REPO_NAME", "My F-Droid Repo"),
		RepoDescription: get("REPO_DESCRIPTION", "Self-hosted F-Droid repository"),
		UploadToken:     getenv("UPLOAD_TOKEN"),
		KeystorePass:    getenv("KEYSTORE_PASS"),
		FdroidBin:       get("FDROID_BIN", "fdroid"),
		KeytoolBin:      get("KEYTOOL_BIN", "keytool"),
		ApksignerPath:   get("APKSIGNER_PATH", "/usr/bin/apksigner"),
		PollInterval:    poll,
		MaxUploadBytes:  maxMB << 20,
	}

	u, err := url.Parse(cfg.PublicURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Config{}, fmt.Errorf("PUBLIC_URL must be an http(s) URL, got %q", cfg.PublicURL)
	}
	return cfg, nil
}
