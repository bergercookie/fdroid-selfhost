package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"fdroid-selfhost/internal/config"
	"fdroid-selfhost/internal/repo"
)

const token = "s3cret"

func newTestServer(t *testing.T, uploadToken string) *httptest.Server {
	t.Helper()
	dir := t.TempDir()
	store, err := repo.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	noop := func(context.Context, string, string, ...string) ([]byte, error) { return nil, nil }
	cfg := config.Config{
		PublicURL:      "https://fdroid.example.com",
		RepoName:       "Test repo",
		UploadToken:    uploadToken,
		MaxUploadBytes: 1 << 20,
	}
	updater := repo.NewUpdater(store, dir, "fdroid", time.Hour, noop)
	web := fstest.MapFS{"index.html": {Data: []byte("<h1>ui</h1>")}}
	srv := httptest.NewServer(New(cfg, store, updater, "ABCD", "1.2.3", web).Handler())
	t.Cleanup(srv.Close)
	return srv
}

func fakeAPK(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("AndroidManifest.xml")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte("x"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func do(t *testing.T, method, url, auth string, body []byte) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), method, url, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if auth != "" {
		req.Header.Set("Authorization", "Bearer "+auth)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func TestInfo(t *testing.T) {
	srv := newTestServer(t, token)
	resp := do(t, "GET", srv.URL+"/api/info", "", nil)
	var got info
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.RepoURL != "https://fdroid.example.com/repo" || got.Version != "1.2.3" {
		t.Errorf("info = %+v", got)
	}
	if want := "fdroidrepos://fdroid.example.com/repo?fingerprint=ABCD"; got.FDroidLink != want {
		t.Errorf("FDroidLink = %q, want %q", got.FDroidLink, want)
	}
}

func TestFDroidLinkUsesPlainSchemeForHTTP(t *testing.T) {
	got := fdroidLink("http://192.168.1.5:8080/repo", "AB")
	if want := "fdroidrepo://192.168.1.5:8080/repo?fingerprint=AB"; got != want {
		t.Errorf("fdroidLink = %q, want %q", got, want)
	}
}

func TestUploadListServeDelete(t *testing.T) {
	srv := newTestServer(t, token)
	apk := fakeAPK(t)

	if resp := do(t, "PUT", srv.URL+"/api/apks/app_1.apk", token, apk); resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload status = %d", resp.StatusCode)
	}

	var apks []repo.APK
	if err := json.NewDecoder(do(t, "GET", srv.URL+"/api/apks", "", nil).Body).Decode(&apks); err != nil {
		t.Fatal(err)
	}
	if len(apks) != 1 || apks[0].Name != "app_1.apk" {
		t.Fatalf("list = %v", apks)
	}

	served := do(t, "GET", srv.URL+"/repo/app_1.apk", "", nil)
	if served.StatusCode != http.StatusOK || served.Header.Get("Cache-Control") != "no-cache" {
		t.Errorf("repo file: status %d, cache %q", served.StatusCode, served.Header.Get("Cache-Control"))
	}

	if resp := do(t, "DELETE", srv.URL+"/api/apks/app_1.apk", token, nil); resp.StatusCode != http.StatusNoContent {
		t.Errorf("delete status = %d", resp.StatusCode)
	}
	if resp := do(t, "DELETE", srv.URL+"/api/apks/app_1.apk", token, nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("second delete status = %d", resp.StatusCode)
	}
}

func TestWriteEndpointsRequireToken(t *testing.T) {
	srv := newTestServer(t, token)
	for name, auth := range map[string]string{"missing": "", "wrong": "nope"} {
		t.Run(name, func(t *testing.T) {
			if resp := do(t, "PUT", srv.URL+"/api/apks/a.apk", auth, fakeAPK(t)); resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("upload status = %d", resp.StatusCode)
			}
			if resp := do(t, "DELETE", srv.URL+"/api/apks/a.apk", auth, nil); resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("delete status = %d", resp.StatusCode)
			}
		})
	}
}

func TestWriteEndpointsDisabledWithoutConfiguredToken(t *testing.T) {
	srv := newTestServer(t, "")
	if resp := do(t, "PUT", srv.URL+"/api/apks/a.apk", "", fakeAPK(t)); resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", resp.StatusCode)
	}
}

func TestUploadValidation(t *testing.T) {
	srv := newTestServer(t, token)
	if resp := do(t, "PUT", srv.URL+"/api/apks/a.apk", token, []byte("not an apk")); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("non-APK status = %d", resp.StatusCode)
	}
	if resp := do(t, "PUT", srv.URL+"/api/apks/a.txt", token, fakeAPK(t)); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("bad name status = %d", resp.StatusCode)
	}
	big := append(fakeAPK(t), bytes.Repeat([]byte("x"), 2<<20)...)
	if resp := do(t, "PUT", srv.URL+"/api/apks/a.apk", token, big); resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("oversize status = %d", resp.StatusCode)
	}
}

func TestServesFrontend(t *testing.T) {
	srv := newTestServer(t, token)
	resp := do(t, "GET", srv.URL+"/", "", nil)
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(resp.Body)
	if resp.StatusCode != http.StatusOK || !strings.Contains(buf.String(), "ui") {
		t.Errorf("frontend: status %d, body %q", resp.StatusCode, buf.String())
	}
}
