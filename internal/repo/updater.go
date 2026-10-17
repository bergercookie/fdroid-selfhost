package repo

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

const updateTimeout = 10 * time.Minute

// Status reports the outcome of the most recent index update.
type Status struct {
	State     string    `json:"state"` // "pending", "updating", "ok" or "error"
	LastRun   time.Time `json:"lastRun"`
	LastError string    `json:"lastError,omitempty"`
}

// Updater regenerates the signed F-Droid index whenever the set of APKs
// changes, whether they arrived through the API, rsync, or a plain copy.
type Updater struct {
	store    *Store
	dataDir  string
	fdroid   string
	interval time.Duration
	run      Runner

	mu     sync.Mutex
	status Status
}

func NewUpdater(store *Store, dataDir, fdroidBin string, interval time.Duration, run Runner) *Updater {
	return &Updater{
		store:    store,
		dataDir:  dataDir,
		fdroid:   fdroidBin,
		interval: interval,
		run:      run,
		status:   Status{State: "pending"},
	}
}

// Status returns a snapshot of the latest update result.
func (u *Updater) Status() Status {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.status
}

// Run updates the index once at startup, then again after every change to the
// APK set, until ctx is cancelled. A failed update is not retried until the
// APKs change again; the error is kept in Status and the log.
func (u *Updater) Run(ctx context.Context) {
	ticker := time.NewTicker(u.interval)
	defer ticker.Stop()

	var last string
	first := true
	for {
		sig, err := u.store.Signature()
		if err != nil {
			slog.Error("scanning repo", "err", err)
		} else if first || sig != last {
			u.update(ctx)
			first, last = false, sig
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (u *Updater) update(ctx context.Context) {
	u.setStatus(Status{State: "updating", LastRun: time.Now().UTC()})

	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()
	out, err := u.run(ctx, u.dataDir, u.fdroid, "update", "--create-metadata")

	st := Status{State: "ok", LastRun: time.Now().UTC()}
	if err != nil {
		st.State, st.LastError = "error", err.Error()
		slog.Error("fdroid update failed", "err", err, "output", string(out))
	} else {
		slog.Info("fdroid index updated")
	}
	u.setStatus(st)
}

func (u *Updater) setStatus(s Status) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.status = s
}
