package config

import "testing"

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RepoURL() != "http://localhost:8080/repo" {
		t.Errorf("RepoURL = %q", cfg.RepoURL())
	}
	if cfg.MaxUploadBytes != 500<<20 {
		t.Errorf("MaxUploadBytes = %d", cfg.MaxUploadBytes)
	}
}

func TestLoadTrimsTrailingSlash(t *testing.T) {
	cfg, err := Load(env(map[string]string{"PUBLIC_URL": "https://fdroid.example.com/"}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RepoURL() != "https://fdroid.example.com/repo" {
		t.Errorf("RepoURL = %q", cfg.RepoURL())
	}
}

func TestLoadRejectsBadValues(t *testing.T) {
	cases := map[string]map[string]string{
		"bad url":      {"PUBLIC_URL": "ftp://example.com"},
		"no host":      {"PUBLIC_URL": "https://"},
		"bad interval": {"POLL_INTERVAL": "soon"},
		"bad max":      {"MAX_UPLOAD_MB": "0"},
	}
	for name, vars := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(env(vars)); err == nil {
				t.Error("expected an error")
			}
		})
	}
}
