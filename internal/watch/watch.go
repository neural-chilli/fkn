package watch

import (
	"context"
	"time"
)

type Runner struct {
	repoRoot string
}

type Options struct {
	Paths     []string
	Debounce  time.Duration
	Poll      time.Duration
	OnTrigger func(time.Time) error
}

type snapshot map[string]fileState

type fileState struct {
	modTime time.Time
	size    int64
}

func New(repoRoot string) *Runner {
	return &Runner{repoRoot: repoRoot}
}

func (r *Runner) Run(ctx context.Context, opts Options) error {
	if opts.Poll <= 0 {
		opts.Poll = 250 * time.Millisecond
	}
	if opts.Debounce <= 0 {
		opts.Debounce = 500 * time.Millisecond
	}

	if err := opts.OnTrigger(time.Now()); err != nil {
		return err
	}

	prev, err := r.scan(opts.Paths)
	if err != nil {
		return err
	}

	if err := r.runEvents(ctx, opts, prev); err == nil {
		return nil
	}
	return r.runPolling(ctx, opts, prev)
}

func changed(before, after snapshot) bool {
	if len(before) != len(after) {
		return true
	}
	for path, prev := range before {
		next, ok := after[path]
		if !ok || !prev.modTime.Equal(next.modTime) || prev.size != next.size {
			return true
		}
	}
	return false
}
