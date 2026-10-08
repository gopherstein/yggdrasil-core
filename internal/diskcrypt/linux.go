//go:build linux

package diskcrypt

import "context"

// detect finds the device the data folder is on and looks for a LUKS
// (dm-crypt) layer under it: lsblk lists the device and what it is built
// on, and a "crypt" among them is encryption.
func detect(ctx context.Context, path string) Status {
	source, err := run(ctx, "findmnt", "-n", "-o", "SOURCE", "--target", path)
	if err != nil || source == "" {
		return Status{State: Unknown}
	}
	types, err := run(ctx, "lsblk", "-s", "-n", "-o", "TYPE", firstLine(source))
	if err != nil {
		return Status{State: Unknown}
	}
	return parseLsblk(types)
}
