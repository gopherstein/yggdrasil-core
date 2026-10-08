//go:build darwin

package diskcrypt

import "context"

// detect reads FileVault's state, which covers the startup disk, where the
// data folder is. fdesetup status needs no administrator.
func detect(ctx context.Context, _ string) Status {
	out, err := run(ctx, "/usr/bin/fdesetup", "status")
	if err != nil {
		return Status{State: Unknown}
	}
	return parseFileVault(out)
}
