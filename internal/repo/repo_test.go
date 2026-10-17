package repo

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"fdroid-selfhost/internal/config"
)

// fakeAPK builds the smallest zip that passes APK validation.
func fakeAPK(t *testing.T, withManifest bool) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	name := "classes.dex"
	if withManifest {
		name = "AndroidManifest.xml"
	}
	w, err := zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte("x"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestStoreSaveListDelete(t *testing.T) {
	s, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Save("app_1.apk", bytes.NewReader(fakeAPK(t, true)), 1<<20); err != nil {
		t.Fatalf("Save: %v", err)
	}
	apks, err := s.List()
	if err != nil || len(apks) != 1 || apks[0].Name != "app_1.apk" {
		t.Fatalf("List = %v, %v", apks, err)
	}
	if err := s.Delete("app_1.apk"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := s.Delete("app_1.apk"); !errors.Is(err, ErrNotFound) {
		t.Errorf("second Delete = %v, want ErrNotFound", err)
	}
}

func TestStoreSaveRejects(t *testing.T) {
	s, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	good := fakeAPK(t, true)
	cases := []struct {
		name string
		file string
		body []byte
		max  int64
		want error
	}{
		{"path traversal", "../evil.apk", good, 1 << 20, ErrInvalidName},
		{"wrong extension", "app.zip", good, 1 << 20, ErrInvalidName},
		{"not a zip", "app.apk", []byte("hello"), 1 << 20, ErrNotAPK},
		{"zip without manifest", "app.apk", fakeAPK(t, false), 1 << 20, ErrNotAPK},
		{"too large", "app.apk", good, 10, ErrTooLarge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := s.Save(tc.file, bytes.NewReader(tc.body), tc.max); !errors.Is(err, tc.want) {
				t.Errorf("Save = %v, want %v", err, tc.want)
			}
		})
	}
	if apks, _ := s.List(); len(apks) != 0 {
		t.Errorf("rejected uploads left files behind: %v", apks)
	}
}

const sampleKeytool = `Alias name: repokey
Certificate fingerprints:
	 SHA1: 11:22:33:44:55:66:77:88:99:AA:BB:CC:DD:EE:FF:00:11:22:33:44
	 SHA256: 0A:1B:2C:3D:4E:5F:60:71:82:93:A4:B5:C6:D7:E8:F9:0A:1B:2C:3D:4E:5F:60:71:82:93:A4:B5:C6:D7:E8:F9
Signature algorithm name: SHA256withRSA
`

func TestParseFingerprint(t *testing.T) {
	got, err := ParseFingerprint(sampleKeytool)
	want := strings.Repeat("0A1B2C3D4E5F60718293A4B5C6D7E8F9", 2)
	if err != nil || got != want {
		t.Errorf("ParseFingerprint = %q, %v; want %q", got, err, want)
	}
	if _, err := ParseFingerprint("nothing here"); err == nil {
		t.Error("expected an error for output without a fingerprint")
	}
}

func TestRenderFdroidConfigQuotesValues(t *testing.T) {
	cfg := config.Config{PublicURL: "https://f.example.com", RepoName: `My "repo": #1`}
	yml := renderFdroidConfig(cfg, "secret", "/usr/bin/apksigner")
	for _, want := range []string{
		`repo_url: "https://f.example.com/repo"`,
		`repo_name: "My \"repo\": #1"`,
		`keystorepass: "secret"`,
		`apksigner: "/usr/bin/apksigner"`,
	} {
		if !strings.Contains(yml, want) {
			t.Errorf("config.yml is missing %q:\n%s", want, yml)
		}
	}
}

func TestSetupCreatesKeyOnceAndReusesIt(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{DataDir: dir, PublicURL: "http://localhost:8080", KeytoolBin: "keytool"}

	genCalls := 0
	run := func(_ context.Context, d, _ string, args ...string) ([]byte, error) {
		if args[0] == "-genkeypair" {
			genCalls++
			return nil, os.WriteFile(filepath.Join(d, keystoreFile), []byte("ks"), 0o600)
		}
		return []byte(sampleKeytool), nil
	}

	for range 2 {
		fp, err := Setup(context.Background(), cfg, run)
		if err != nil || len(fp) != 64 {
			t.Fatalf("Setup = %q, %v", fp, err)
		}
	}
	if genCalls != 1 {
		t.Errorf("keystore generated %d times, want 1", genCalls)
	}
	if _, err := os.Stat(filepath.Join(dir, configFile)); err != nil {
		t.Errorf("config.yml not written: %v", err)
	}
}

func TestSetupFailsWhenKeystorePasswordIsLost(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, keystoreFile), []byte("ks"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{DataDir: dir, PublicURL: "http://localhost:8080"}
	if _, err := Setup(context.Background(), cfg, nil); err == nil {
		t.Error("expected an error when the keystore password is unknown")
	}
}

func TestUpdaterRunsOnStartAndOnChange(t *testing.T) {
	s, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	calls := 0
	run := func(context.Context, string, string, ...string) ([]byte, error) {
		mu.Lock()
		defer mu.Unlock()
		calls++
		return nil, nil
	}
	count := func() int {
		mu.Lock()
		defer mu.Unlock()
		return calls
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go NewUpdater(s, t.TempDir(), "fdroid", 10*time.Millisecond, run).Run(ctx)

	waitFor(t, func() bool { return count() == 1 }, "initial update")
	time.Sleep(60 * time.Millisecond)
	if count() != 1 {
		t.Fatalf("updater ran %d times without any change", count())
	}
	if err := s.Save("a.apk", bytes.NewReader(fakeAPK(t, true)), 1<<20); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return count() == 2 }, "update after upload")
}

func waitFor(t *testing.T, cond func() bool, what string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}
