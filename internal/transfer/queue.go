package transfer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Direction string
type JobStatus string

const (
	Upload   Direction = "upload"
	Download Direction = "download"

	StatusPending   JobStatus = "pending"
	StatusRunning   JobStatus = "running"
	StatusDone      JobStatus = "done"
	StatusFailed    JobStatus = "failed"
	StatusCancelled JobStatus = "cancelled"
	// StatusPaused: put aside ("resume later", lost connection, app closed).
	// It resumes on the next connection to the same server account.
	StatusPaused JobStatus = "paused"
)

type Job struct {
	ID         string    `json:"id"`
	Direction  Direction `json:"direction"`
	LocalPath  string    `json:"localPath"`
	RemotePath string    `json:"remotePath"`
	RemoteHost string    `json:"remoteHost"`
	// ConnID is the connection the job was queued on. The job always runs
	// on it, whatever tab is active when it starts (see Resolver).
	ConnID string `json:"connId"`
	// Account is the server account (protocol://user@host:port, same key as
	// the frontend's favKey): paused jobs resume on a new connection to it.
	Account    string    `json:"account"`
	Name       string    `json:"name"`
	Size       int64     `json:"size"`
	BytesDone  int64     `json:"bytesDone"`
	Status     JobStatus `json:"status"`
	Error      string    `json:"error"`
	CreatedAt  time.Time `json:"createdAt"`
	FinishedAt time.Time `json:"finishedAt"`
	// SrcModTime is the local file's modification time when an upload
	// started from zero; a resume restarts from zero if it (or Size) changed.
	SrcModTime time.Time `json:"srcModTime"`
	// Resume: the next run continues after what is already at the
	// destination. Only set when this job wrote part of it (BytesDone > 0)
	// or on an explicit "Resume" - never because a file happens to be there
	// (a "Replace" paused before its first byte must not resume an old file).
	Resume   bool               `json:"resume"`
	cancelFn context.CancelFunc `json:"-"`
	// stopAs is set when the job is interrupted on purpose (connection
	// closed, paused): the status it ends with instead of "cancelled".
	stopAs JobStatus
}

// Target is the connection a job runs on.
type Target struct {
	ConnID  string
	Host    string
	Account string
}

type Executor interface {
	UploadFrom(ctx context.Context, localPath, remotePath string, offset int64, progress func(sent, total int64)) error
	DownloadFrom(ctx context.Context, remotePath, localPath string, offset int64, progress func(received, total int64)) error
	RemoteSize(path string) (int64, error)
}

type EventEmitter func(eventName string, data interface{})

// Resolver returns the executor of a connection, or nil once it is closed.
type Resolver func(connID string) Executor

// ErrConnectionClosed is the job error when its connection was closed before
// or while it ran; the frontend translates it.
const ErrConnectionClosed = "connection_closed"

type Queue struct {
	mu            sync.Mutex
	jobs          []*Job
	workers       int
	sem           chan struct{}
	emitter       EventEmitter
	resolve       Resolver
	ctx           context.Context
	cancel        context.CancelFunc
	speedLimitBps int64 // bytes/sec, 0 = unlimited
	// generation is bumped by CancelAll. Folder walkers (QueueUploadDir /
	// QueueDownloadDir) capture it when they start and stop adding jobs as
	// soon as it changes - see AddForGeneration.
	generation uint64
	// storePath is the file the unfinished jobs are saved to (see store.go);
	// dirty means it is out of date.
	storePath string
	dirty     bool
	saveMu    sync.Mutex // one Save at a time (flusher vs shutdown)
}

func NewQueue(workers int, emitter EventEmitter) *Queue {
	ctx, cancel := context.WithCancel(context.Background())
	return &Queue{
		workers: workers,
		sem:     make(chan struct{}, workers),
		emitter: emitter,
		ctx:     ctx,
		cancel:  cancel,
	}
}

func (q *Queue) SetResolver(r Resolver) {
	q.mu.Lock()
	q.resolve = r
	q.mu.Unlock()
}

func (q *Queue) SetSpeedLimit(kbps int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.speedLimitBps = int64(kbps) * 1024
}

// Add queues a transfer on t. resume = continue after what is already at
// the destination (the conflict dialog's "Resume").
func (q *Queue) Add(dir Direction, localPath, remotePath string, t Target, resume bool) *Job {
	return q.add(dir, localPath, remotePath, t, resume, nil)
}

// Generation returns the current cancel-all generation (see CancelAll).
func (q *Queue) Generation() uint64 {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.generation
}

// AddForGeneration adds a job only if no CancelAll happened since gen was
// read; returns nil otherwise so the caller (a folder walker) can stop.
// The check and the append happen under the same lock, so no job can slip
// in after a CancelAll.
func (q *Queue) AddForGeneration(gen uint64, dir Direction, localPath, remotePath string, t Target) *Job {
	return q.add(dir, localPath, remotePath, t, false, &gen)
}

func (q *Queue) add(dir Direction, localPath, remotePath string, t Target, resume bool, gen *uint64) *Job {
	job := &Job{
		ID:         uuid.New().String(),
		Direction:  dir,
		LocalPath:  localPath,
		RemotePath: remotePath,
		RemoteHost: t.Host,
		ConnID:     t.ConnID,
		Account:    t.Account,
		Name:       filepath.Base(localPath),
		Status:     StatusPending,
		CreatedAt:  time.Now(),
		Resume:     resume,
	}
	if dir == Download {
		job.Name = filepath.Base(remotePath)
	}

	q.mu.Lock()
	if gen != nil && *gen != q.generation {
		q.mu.Unlock()
		return nil
	}
	q.jobs = append(q.jobs, job)
	q.dirty = true
	q.mu.Unlock()

	q.emitter("transfer:added", job)
	go q.run(job)
	return job
}

func (q *Queue) SetWorkers(n int) {
	q.mu.Lock()
	q.workers = n
	q.sem = make(chan struct{}, n)
	q.mu.Unlock()
}

func (q *Queue) run(job *Job) {
	// Snapshot sem so acquire and release use the same channel even if SetWorkers replaces it.
	q.mu.Lock()
	sem := q.sem
	q.mu.Unlock()

	select {
	case sem <- struct{}{}:
	case <-q.ctx.Done():
		q.setStatus(job, StatusCancelled, "")
		return
	}
	defer func() { <-sem }()

	// The job may have been cancelled, failed (connection closed) or paused
	// while it was waiting for a worker slot - don't start it in that case.
	q.mu.Lock()
	stopped := job.Status != StatusPending
	q.mu.Unlock()
	if stopped {
		return
	}

	jobCtx, jobCancel := context.WithCancel(q.ctx)
	q.mu.Lock()
	job.cancelFn = jobCancel
	q.mu.Unlock()
	defer jobCancel()

	q.setStatus(job, StatusRunning, "")

	q.mu.Lock()
	resolve := q.resolve
	resume := job.Resume
	q.mu.Unlock()

	var executor Executor
	if resolve != nil {
		executor = resolve(job.ConnID)
	}
	if executor == nil {
		q.setStatus(job, StatusFailed, ErrConnectionClosed)
		return
	}

	var offset int64
	if resume {
		offset = resumeOffset(job, executor)
		if offset < 0 {
			// The destination is already complete (the cut happened at
			// the very end): nothing left to send.
			q.mu.Lock()
			job.Resume = false
			job.BytesDone = job.Size
			q.mu.Unlock()
			q.setStatus(job, StatusDone, "")
			return
		}
	}
	q.mu.Lock()
	job.Resume = false
	job.BytesDone = offset
	q.mu.Unlock()

	if job.Direction == Upload {
		if info, err := os.Stat(job.LocalPath); err == nil {
			q.mu.Lock()
			job.Size = info.Size()
			if offset == 0 || job.SrcModTime.IsZero() {
				job.SrcModTime = info.ModTime()
			}
			q.mu.Unlock()
		}
	}

	var err error
	jobStart := time.Now()
	progress := func(done, total int64) {
		q.mu.Lock()
		job.BytesDone = done
		job.Size = total
		limitBps := q.speedLimitBps
		q.dirty = true // the saved queue keeps how far it got (crash)
		q.mu.Unlock()

		// Only the bytes sent in this run count for the speed limit, not
		// the part already there before a resume.
		if sent := done - offset; limitBps > 0 && sent > 0 {
			elapsed := time.Since(jobStart).Seconds()
			expected := float64(sent) / float64(limitBps)
			if expected > elapsed {
				time.Sleep(time.Duration((expected - elapsed) * float64(time.Second)))
			}
		}

		q.emitter("transfer:progress", job)
	}

	if job.Direction == Upload {
		err = executor.UploadFrom(jobCtx, job.LocalPath, job.RemotePath, offset, progress)
	} else {
		err = executor.DownloadFrom(jobCtx, job.RemotePath, job.LocalPath, offset, progress)
	}

	if err != nil {
		q.mu.Lock()
		stopAs := job.stopAs
		job.stopAs = ""
		q.mu.Unlock()
		switch {
		case stopAs == StatusPaused:
			q.mu.Lock()
			job.Resume = job.BytesDone > 0
			q.mu.Unlock()
			q.setStatus(job, StatusPaused, "")
		case stopAs == StatusFailed || resolve(job.ConnID) == nil:
			// The connection went away during the transfer (closed tab,
			// lost connection): one clear reason instead of a raw EOF.
			q.setStatus(job, StatusFailed, ErrConnectionClosed)
		case errors.Is(err, context.Canceled):
			q.setStatus(job, StatusCancelled, "")
		default:
			q.setStatus(job, StatusFailed, err.Error())
		}
	} else {
		q.mu.Lock()
		job.BytesDone = job.Size
		job.FinishedAt = time.Now()
		q.mu.Unlock()
		q.setStatus(job, StatusDone, "")
	}
}

// resumeOffset returns where a resumed job continues: the size of the
// partial destination file, 0 to start again from zero (no partial file,
// source changed since the first attempt, destination larger than the
// source, size unknown), or -1 when the destination is already complete.
func resumeOffset(job *Job, ex Executor) int64 {
	var src, dst int64
	if job.Direction == Upload {
		info, err := os.Stat(job.LocalPath)
		if err != nil {
			return 0
		}
		if job.Size > 0 && info.Size() != job.Size {
			return 0
		}
		if !job.SrcModTime.IsZero() && info.ModTime().Unix() != job.SrcModTime.Unix() {
			return 0
		}
		src = info.Size()
		if dst, err = ex.RemoteSize(job.RemotePath); err != nil {
			return 0
		}
	} else {
		var err error
		if src, err = ex.RemoteSize(job.RemotePath); err != nil {
			return 0
		}
		if job.Size > 0 && src != job.Size {
			return 0
		}
		info, err := os.Stat(job.LocalPath)
		if err != nil {
			return 0
		}
		dst = info.Size()
	}
	switch {
	case src <= 0 || dst <= 0 || dst > src:
		return 0
	case dst == src:
		return -1
	}
	return dst
}

func (q *Queue) setStatus(job *Job, status JobStatus, errMsg string) {
	q.mu.Lock()
	job.Status = status
	job.Error = errMsg
	if status == StatusDone || status == StatusFailed || status == StatusCancelled {
		job.FinishedAt = time.Now()
	}
	q.dirty = true
	q.mu.Unlock()
	q.emitter("transfer:update", job)
}

func (q *Queue) GetAll() []*Job {
	q.mu.Lock()
	defer q.mu.Unlock()
	result := make([]*Job, len(q.jobs))
	copy(result, q.jobs)
	return result
}

// Get returns a copy of a job.
func (q *Queue) Get(id string) (Job, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, job := range q.jobs {
		if job.ID == id {
			return *job, true
		}
	}
	return Job{}, false
}

// ActiveCount is the number of pending and running jobs.
func (q *Queue) ActiveCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	n := 0
	for _, job := range q.jobs {
		if job.Status == StatusPending || job.Status == StatusRunning {
			n++
		}
	}
	return n
}

func (q *Queue) Cancel(id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, job := range q.jobs {
		if job.ID == id {
			if job.Status == StatusPending || job.Status == StatusPaused {
				job.Status = StatusCancelled
				job.FinishedAt = time.Now()
				q.dirty = true
				q.emitter("transfer:update", job)
				return nil
			}
			if job.Status == StatusRunning && job.cancelFn != nil {
				job.cancelFn()
				return nil
			}
		}
	}
	return fmt.Errorf("job not found or not cancellable")
}

// CancelAll cancels every pending, paused and running job and stops
// in-progress folder walkers from queueing more (generation bump). Pending
// jobs are reported in one "transfer:cancelledAll" event (ids) rather than
// one event per job - a dropped folder can hold thousands of files. Running
// jobs emit their own "transfer:update" once their transfer is interrupted.
func (q *Queue) CancelAll() {
	q.mu.Lock()
	q.generation++
	now := time.Now()
	var ids []string
	var cancelFns []context.CancelFunc
	for _, job := range q.jobs {
		switch job.Status {
		case StatusPending, StatusPaused:
			job.Status = StatusCancelled
			job.FinishedAt = now
			ids = append(ids, job.ID)
		case StatusRunning:
			if job.cancelFn != nil {
				cancelFns = append(cancelFns, job.cancelFn)
			}
		}
	}
	q.dirty = true
	q.mu.Unlock()

	for _, cancel := range cancelFns {
		cancel()
	}
	q.emitter("transfer:cancelledAll", ids)
}

// stop ends the pending and running jobs matching match: as StatusFailed
// (ErrConnectionClosed) or StatusPaused (resumed later). Pending jobs change
// at once and are reported in one "transfer:stopped" event; running ones are
// interrupted and emit their own update.
func (q *Queue) stop(match func(*Job) bool, as JobStatus) {
	q.mu.Lock()
	now := time.Now()
	var ids []string
	var cancelFns []context.CancelFunc
	for _, job := range q.jobs {
		if !match(job) {
			continue
		}
		switch job.Status {
		case StatusPending:
			job.Status = as
			if as == StatusFailed {
				job.Error = ErrConnectionClosed
				job.FinishedAt = now
			} else {
				job.Resume = job.Resume || job.BytesDone > 0
			}
			ids = append(ids, job.ID)
		case StatusRunning:
			// The first reason wins: "resume later" chosen before the
			// connection is closed must not turn into a failure.
			if job.stopAs == "" {
				job.stopAs = as
			}
			if job.cancelFn != nil {
				cancelFns = append(cancelFns, job.cancelFn)
			}
		}
	}
	q.dirty = true
	q.mu.Unlock()

	for _, cancel := range cancelFns {
		cancel()
	}
	if len(ids) > 0 {
		errMsg := ""
		if as == StatusFailed {
			errMsg = ErrConnectionClosed
		}
		q.emitter("transfer:stopped", map[string]interface{}{"ids": ids, "status": as, "error": errMsg})
	}
}

func stopStatus(pause bool) JobStatus {
	if pause {
		return StatusPaused
	}
	return StatusFailed
}

// ConnectionClosed ends the jobs of a connection that is being closed:
// paused (pause) or failed with ErrConnectionClosed. They never move to
// another connection.
func (q *Queue) ConnectionClosed(connID string, pause bool) {
	q.stop(func(j *Job) bool { return j.ConnID == connID }, stopStatus(pause))
}

// StopConnections is ConnectionClosed for several connections, before they
// are closed (the frontend's "stop" / "resume later" choice).
func (q *Queue) StopConnections(connIDs []string, pause bool) {
	set := make(map[string]bool, len(connIDs))
	for _, id := range connIDs {
		set[id] = true
	}
	q.stop(func(j *Job) bool { return set[j.ConnID] }, stopStatus(pause))
}

// StopAll stops every pending and running job (quitting the app).
func (q *Queue) StopAll(pause bool) {
	q.stop(func(*Job) bool { return true }, stopStatus(pause))
}

// ResumePaused restarts the paused jobs of t's account on connection t.
// Returns how many were restarted; one "transfer:requeued" event.
func (q *Queue) ResumePaused(t Target) int {
	q.mu.Lock()
	var jobs []*Job
	for _, job := range q.jobs {
		if job.Status == StatusPaused && job.Account == t.Account {
			job.Status = StatusPending
			job.ConnID = t.ConnID
			job.RemoteHost = t.Host
			job.Error = ""
			jobs = append(jobs, job)
		}
	}
	q.dirty = true
	q.mu.Unlock()

	if len(jobs) == 0 {
		return 0
	}
	ids := make([]string, len(jobs))
	for i, job := range jobs {
		ids[i] = job.ID
	}
	q.emitter("transfer:requeued", map[string]interface{}{"ids": ids, "connId": t.ConnID, "remoteHost": t.Host})
	for _, job := range jobs {
		go q.run(job)
	}
	return len(jobs)
}

func (q *Queue) Clear(status JobStatus) {
	q.mu.Lock()
	defer q.mu.Unlock()
	var remaining []*Job
	for _, job := range q.jobs {
		if job.Status != status {
			remaining = append(remaining, job)
		}
	}
	q.jobs = remaining
	q.dirty = true
	q.emitter("transfer:cleared", status)
}

func (q *Queue) RemoveJob(id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i, job := range q.jobs {
		if job.ID == id && job.Status != StatusRunning && job.Status != StatusPending {
			q.jobs = append(q.jobs[:i], q.jobs[i+1:]...)
			q.dirty = true
			q.emitter("transfer:removed", id)
			return nil
		}
	}
	return fmt.Errorf("job not found or still active")
}

// Retry runs a failed or paused job again on t: resume = continue after
// what is already at the destination, else start again from zero.
func (q *Queue) Retry(id string, resume bool, t Target) error {
	q.mu.Lock()
	var found *Job
	for _, job := range q.jobs {
		if job.ID == id && (job.Status == StatusFailed || job.Status == StatusPaused) {
			found = job
			break
		}
	}
	if found == nil {
		q.mu.Unlock()
		return fmt.Errorf("job not found or not retryable")
	}
	found.Status = StatusPending
	found.Error = ""
	found.ConnID = t.ConnID
	found.RemoteHost = t.Host
	found.Resume = resume && (found.Resume || found.BytesDone > 0)
	if !resume {
		found.BytesDone = 0
		found.SrcModTime = time.Time{}
	}
	found.FinishedAt = time.Time{}
	q.dirty = true
	q.mu.Unlock()
	q.emitter("transfer:update", found)
	go q.run(found)
	return nil
}
