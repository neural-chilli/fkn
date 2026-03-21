package runner

import "fmt"

func requireSafetyApproval(taskName string, safety string, opts Options) error {
	if opts.DryRun || opts.AllowUnsafe {
		return nil
	}
	switch safety {
	case "destructive", "external":
		return fmt.Errorf("task %q is marked %s; rerun with --allow-unsafe or allow_unsafe=true to execute it", taskName, safety)
	default:
		return nil
	}
}
