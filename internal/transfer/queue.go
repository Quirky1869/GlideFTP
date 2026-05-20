package transfer

import (
	"context"
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

	StatusPending    JobStatus = "pending"
	StatusRunning    JobStatus = "running"
	StatusDone       JobStatus = "done"
	StatusFailed     JobStatus = "failed"
	StatusCancelled  JobStatus = "cancelled"
)

type Job struct {
	ID         string    `json:"id"`
	Direction  Direction `json:"direction"`
	LocalPath  string    `json:"localPath"`
	RemotePath string    `json:"remotePath"`
	Name       string    `json:"name"`
	Size       int64     `json:"size"`
	BytesDone  int64     `json:"bytesDone"`
	Status     JobStatus `json:"status"`
	Error      string    `json:"error"`
	CreatedAt  time.Time `json:"createdAt"`
	FinishedAt time.Time `json:"finishedAt"`
}

type Executor interface {
	Upload(localPath, remotePath string, progress func(sent, total int64)) error
	Download(remotePath, localPath string, progress func(received, total int64)) error
}

type EventEmitter func(eventName string, data interface{})

type Queue struct {
	mu       sync.Mutex
	jobs     []*Job
	workers  int
	sem      chan struct{}
	emitter  EventEmitter
	executor Executor
	ctx      context.Context
	cancel   context.CancelFunc
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

func (q *Queue) SetExecutor(e Executor) {
	q.mu.Lock()
	q.executor = e
	q.mu.Unlock()
}

func (q *Queue) Add(dir Direction, localPath, remotePath string) *Job {
	job := &Job{
		ID:         uuid.New().String(),
		Direction:  dir,
		LocalPath:  localPath,
		RemotePath: remotePath,
		Name:       filepath.Base(localPath),
		Status:     StatusPending,
		CreatedAt:  time.Now(),
	}
	if dir == Download {
		job.Name = filepath.Base(remotePath)
	}

	q.mu.Lock()
	q.jobs = append(q.jobs, job)
	q.mu.Unlock()

	q.emitter("transfer:added", job)
	go q.run(job)
	return job
}

func (q *Queue) run(job *Job) {
	select {
	case q.sem <- struct{}{}:
	case <-q.ctx.Done():
		q.setStatus(job, StatusCancelled, "")
		return
	}
	defer func() { <-q.sem }()

	q.setStatus(job, StatusRunning, "")

	q.mu.Lock()
	executor := q.executor
	q.mu.Unlock()

	if executor == nil {
		q.setStatus(job, StatusFailed, "no active connection")
		return
	}

	var err error
	progress := func(done, total int64) {
		q.mu.Lock()
		job.BytesDone = done
		job.Size = total
		q.mu.Unlock()
		q.emitter("transfer:progress", job)
	}

	if job.Direction == Upload {
		info, statErr := os.Stat(job.LocalPath)
		if statErr == nil {
			q.mu.Lock()
			job.Size = info.Size()
			q.mu.Unlock()
		}
		err = executor.Upload(job.LocalPath, job.RemotePath, progress)
	} else {
		err = executor.Download(job.RemotePath, job.LocalPath, progress)
	}

	if err != nil {
		q.setStatus(job, StatusFailed, err.Error())
	} else {
		q.mu.Lock()
		job.BytesDone = job.Size
		job.FinishedAt = time.Now()
		q.mu.Unlock()
		q.setStatus(job, StatusDone, "")
	}
}

func (q *Queue) setStatus(job *Job, status JobStatus, errMsg string) {
	q.mu.Lock()
	job.Status = status
	job.Error = errMsg
	if status == StatusDone || status == StatusFailed || status == StatusCancelled {
		job.FinishedAt = time.Now()
	}
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

func (q *Queue) Cancel(id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, job := range q.jobs {
		if job.ID == id && job.Status == StatusPending {
			job.Status = StatusCancelled
			job.FinishedAt = time.Now()
			q.emitter("transfer:update", job)
			return nil
		}
	}
	return fmt.Errorf("job not found or not cancellable")
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
	q.emitter("transfer:cleared", status)
}

func (q *Queue) Retry(id string) error {
	q.mu.Lock()
	var found *Job
	for _, job := range q.jobs {
		if job.ID == id && job.Status == StatusFailed {
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
	found.BytesDone = 0
	found.FinishedAt = time.Time{}
	q.mu.Unlock()
	go q.run(found)
	return nil
}
