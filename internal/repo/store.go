// Package repo manages the F-Droid repository on disk: the APK files, the
// signing key, and running fdroidserver to regenerate the signed index.
package repo

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"
)

var (
	ErrInvalidName = errors.New("invalid file name: use letters, digits, '.', '_' or '-' and end with .apk")
	ErrNotAPK      = errors.New("file is not an APK (no AndroidManifest.xml inside)")
	ErrTooLarge    = errors.New("file is too large")
	ErrNotFound    = errors.New("apk not found")
)

var nameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}\.apk$`)

// ValidName reports whether name is safe to use as a file name in the repo.
func ValidName(name string) bool { return nameRe.MatchString(name) }

// APK describes one file in the repository.
type APK struct {
	Name     string    `json:"name"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
}

// Store reads and writes APK files in <dataDir>/repo.
type Store struct {
	repoDir string
	tmpDir  string
}

// NewStore creates the repo and temporary directories if needed.
func NewStore(dataDir string) (*Store, error) {
	s := &Store{
		repoDir: filepath.Join(dataDir, "repo"),
		tmpDir:  filepath.Join(dataDir, "tmp"),
	}
	for _, dir := range []string{s.repoDir, s.tmpDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // repo files are public by design
			return nil, err
		}
	}
	return s, nil
}

// RepoDir is the directory served to F-Droid clients.
func (s *Store) RepoDir() string { return s.repoDir }

// Save validates body as an APK and atomically places it in the repo,
// replacing any existing file with the same name.
func (s *Store) Save(name string, body io.Reader, maxBytes int64) error {
	if !ValidName(name) {
		return ErrInvalidName
	}
	tmp, err := os.CreateTemp(s.tmpDir, "upload-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // harmless after a successful rename

	n, err := io.Copy(tmp, io.LimitReader(body, maxBytes+1))
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if n > maxBytes {
		return ErrTooLarge
	}
	if err := checkAPK(tmp.Name()); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil { //nolint:gosec // repo files are public by design
		return err
	}
	return os.Rename(tmp.Name(), filepath.Join(s.repoDir, name))
}

// List returns the APKs in the repo, sorted by name.
func (s *Store) List() ([]APK, error) {
	entries, err := os.ReadDir(s.repoDir)
	if err != nil {
		return nil, err
	}
	apks := []APK{}
	for _, e := range entries {
		if e.IsDir() || !ValidName(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return nil, err
		}
		apks = append(apks, APK{Name: e.Name(), Size: info.Size(), Modified: info.ModTime().UTC()})
	}
	sort.Slice(apks, func(i, j int) bool { return apks[i].Name < apks[j].Name })
	return apks, nil
}

// Delete removes an APK from the repo.
func (s *Store) Delete(name string) error {
	if !ValidName(name) {
		return ErrInvalidName
	}
	err := os.Remove(filepath.Join(s.repoDir, name))
	if errors.Is(err, os.ErrNotExist) {
		return ErrNotFound
	}
	return err
}

// Signature changes whenever an APK is added, removed or replaced.
func (s *Store) Signature() (string, error) {
	apks, err := s.List()
	if err != nil {
		return "", err
	}
	sig := ""
	for _, a := range apks {
		sig += fmt.Sprintf("%s|%d|%d\n", a.Name, a.Size, a.Modified.UnixNano())
	}
	return sig, nil
}

func checkAPK(path string) error {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return ErrNotAPK
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name == "AndroidManifest.xml" {
			return nil
		}
	}
	return ErrNotAPK
}
