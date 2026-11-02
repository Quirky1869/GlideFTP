package transfer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// The unfinished jobs (pending, running, paused, failed) are saved to
// queue.json in the config folder, so they survive a restart or a crash:
// at the next launch they come back paused (failed ones stay failed) and
// resume on the next connection to their server account. Done and
// cancelled jobs are not kept.

const storeFlushInterval = 2 * time.Second

// EnableStore loads the jobs saved at path, then keeps the file up to date
// (every storeFlushInterval while something changed, and on Save).
func (q *Queue) EnableStore(path string) {
	q.mu.Lock()
	q.storePath = path
	q.mu.Unlock()
	q.load(path)
	go func() {
		t := time.NewTicker(storeFlushInterval)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				q.mu.Lock()
				dirty := q.dirty
				q.mu.Unlock()
				if dirty {
					q.Save()
				}
			case <-q.ctx.Done():
				return
			}
		}
	}()
}

func (q *Queue) load(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var saved []*Job
	if json.Unmarshal(data, &saved) != nil {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, job := range saved {
		if job == nil || job.ID == "" {
			continue
		}
		switch job.Status {
		case StatusPending, StatusRunning, StatusPaused:
			job.Resume = job.Resume || job.BytesDone > 0
			job.Status = StatusPaused
		case StatusFailed:
		default:
			continue
		}
		// Connections don't survive a restart: the job is bound again when
		// it is resumed (ResumePaused / Retry).
		job.ConnID = ""
		q.jobs = append(q.jobs, job)
	}
}

// Save writes the unfinished jobs now (also called at shutdown). A running
// job is saved as paused, or failed when it is being stopped as failed.
func (q *Queue) Save() error {
	q.saveMu.Lock()
	defer q.saveMu.Unlock()
	q.mu.Lock()
	path := q.storePath
	if path == "" {
		q.mu.Unlock()
		return nil
	}
	saved := make([]Job, 0, len(q.jobs))
	for _, job := range q.jobs {
		j := *job
		switch j.Status {
		case StatusRunning:
			if j.stopAs == StatusFailed {
				j.Status = StatusFailed
				j.Error = ErrConnectionClosed
			} else {
				j.Status = StatusPaused
				j.Resume = j.Resume || j.BytesDone > 0
			}
		case StatusPending, StatusPaused, StatusFailed:
		default:
			continue
		}
		saved = append(saved, j)
	}
	q.dirty = false
	q.mu.Unlock()

	data, err := json.Marshal(saved)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	// Temp file + rename: a crash while writing never leaves half a file.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
