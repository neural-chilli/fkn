package watch

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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

	ticker := time.NewTicker(opts.Poll)
	defer ticker.Stop()

	var pending bool
	var deadline time.Time

	for {
		select {
		case <-ctx.Done():
			return nil
		case now := <-ticker.C:
			current, err := r.scan(opts.Paths)
			if err != nil {
				return err
			}
			if changed(prev, current) {
				pending = true
				deadline = now.Add(opts.Debounce)
			}
			prev = current

			if pending && !now.Before(deadline) {
				pending = false
				if err := opts.OnTrigger(now); err != nil {
					return err
				}
				prev, err = r.scan(opts.Paths)
				if err != nil {
					return err
				}
			}
		}
	}
}

func (r *Runner) scan(paths []string) (snapshot, error) {
	if len(paths) == 0 {
		paths = []string{"."}
	}

	out := snapshot{}
	for _, pattern := range paths {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}

		absolute := filepath.Join(r.repoRoot, pattern)
		if info, err := os.Stat(absolute); err == nil {
			if info.IsDir() {
				if err := filepath.WalkDir(absolute, func(path string, d os.DirEntry, err error) error {
					if err != nil {
						return nil
					}
					rel, relErr := filepath.Rel(r.repoRoot, path)
					if relErr != nil || shouldIgnore(rel) {
						if d.IsDir() && relErr == nil && shouldIgnore(rel) {
							return filepath.SkipDir
						}
						return nil
					}
					if d.IsDir() {
						return nil
					}
					info, statErr := d.Info()
					if statErr != nil {
						return nil
					}
					out[filepath.ToSlash(rel)] = fileState{modTime: info.ModTime(), size: info.Size()}
					return nil
				}); err != nil {
					return nil, err
				}
				continue
			}

			rel, relErr := filepath.Rel(r.repoRoot, absolute)
			if relErr == nil && !shouldIgnore(rel) {
				out[filepath.ToSlash(rel)] = fileState{modTime: info.ModTime(), size: info.Size()}
			}
			continue
		}

		matches, err := filepath.Glob(filepath.Join(r.repoRoot, pattern))
		if err != nil {
			return nil, err
		}
		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil || info.IsDir() {
				continue
			}
			rel, relErr := filepath.Rel(r.repoRoot, match)
			if relErr == nil && !shouldIgnore(rel) {
				out[filepath.ToSlash(rel)] = fileState{modTime: info.ModTime(), size: info.Size()}
			}
		}
	}

	return out, nil
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

func shouldIgnore(rel string) bool {
	rel = filepath.ToSlash(rel)
	return rel == ".git" || strings.HasPrefix(rel, ".git/") ||
		rel == ".fkn" || strings.HasPrefix(rel, ".fkn/") ||
		rel == "bin" || strings.HasPrefix(rel, "bin/")
}
