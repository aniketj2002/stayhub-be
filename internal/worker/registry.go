package worker

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type Job interface {
	Name() string
	Run(ctx context.Context, now time.Time, limit int) (int, error)
}

type Registry struct {
	mu   sync.RWMutex
	jobs []Job
}

func NewRegistry() *Registry {
	return &Registry{
		jobs: make([]Job, 0),
	}
}

func (r *Registry) Register(job Job) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jobs = append(r.jobs, job)
}

func (r *Registry) Jobs() []Job {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Job, len(r.jobs))
	copy(out, r.jobs)
	return out
}

func (r *Registry) RunAll(ctx context.Context, now time.Time, limit int) (map[string]int, error) {
	results := make(map[string]int)
	for _, job := range r.Jobs() {
		count, err := job.Run(ctx, now, limit)
		if err != nil {
			return results, fmt.Errorf("job %s failed: %w", job.Name(), err)
		}
		results[job.Name()] = count
	}
	return results, nil
}
