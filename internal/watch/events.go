package watch

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

func (r *Runner) runEvents(ctx context.Context, opts Options, prev snapshot) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	if err := r.addWatches(watcher, opts.Paths); err != nil {
		return err
	}

	var (
		pending bool
		timer   *time.Timer
		timerC  <-chan time.Time
	)

	resetTimer := func() {
		if timer == nil {
			timer = time.NewTimer(opts.Debounce)
		} else {
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(opts.Debounce)
		}
		timerC = timer.C
	}

	triggerIfChanged := func(now time.Time) error {
		current, err := r.scan(opts.Paths)
		if err != nil {
			return err
		}
		if !changed(prev, current) {
			return nil
		}
		prev = current
		return opts.OnTrigger(now)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-watcher.Errors:
			if err != nil {
				return err
			}
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if shouldIgnoreEvent(event.Name) {
				continue
			}
			if event.Op&(fsnotify.Create|fsnotify.Rename) != 0 {
				_ = r.addPathWatches(watcher, event.Name)
			}
			pending = true
			resetTimer()
		case now := <-timerC:
			if !pending {
				continue
			}
			pending = false
			timerC = nil
			if err := triggerIfChanged(now); err != nil {
				return err
			}
		}
	}
}

func (r *Runner) addWatches(watcher *fsnotify.Watcher, paths []string) error {
	dirs, err := r.watchDirectories(paths)
	if err != nil {
		return err
	}
	for _, dir := range dirs {
		if err := watcher.Add(dir); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runner) watchDirectories(paths []string) ([]string, error) {
	if len(paths) == 0 {
		paths = []string{"."}
	}

	seen := map[string]bool{}
	var dirs []string
	addDir := func(dir string) {
		dir = filepath.Clean(dir)
		if seen[dir] {
			return
		}
		seen[dir] = true
		dirs = append(dirs, dir)
	}

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
					if relErr == nil && shouldIgnore(rel) {
						if d.IsDir() {
							return filepath.SkipDir
						}
						return nil
					}
					if d.IsDir() {
						addDir(path)
					}
					return nil
				}); err != nil {
					return nil, err
				}
				continue
			}
			addDir(filepath.Dir(absolute))
			continue
		}

		matches, err := filepath.Glob(absolute)
		if err != nil {
			return nil, err
		}
		if len(matches) == 0 {
			addDir(filepath.Dir(absolute))
			continue
		}
		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil {
				continue
			}
			if info.IsDir() {
				addDir(match)
				continue
			}
			addDir(filepath.Dir(match))
		}
	}

	if len(dirs) == 0 {
		addDir(r.repoRoot)
	}
	return dirs, nil
}

func (r *Runner) addPathWatches(watcher *fsnotify.Watcher, path string) error {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return nil
	}
	return filepath.WalkDir(path, func(child string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, relErr := filepath.Rel(r.repoRoot, child)
		if relErr == nil && shouldIgnore(rel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if err := watcher.Add(child); err != nil {
				return nil
			}
		}
		return nil
	})
}

func shouldIgnoreEvent(path string) bool {
	rel := filepath.ToSlash(path)
	return shouldIgnore(rel) || strings.Contains(rel, "/.git/") || strings.Contains(rel, "/.fkn/") || strings.Contains(rel, "/bin/")
}
