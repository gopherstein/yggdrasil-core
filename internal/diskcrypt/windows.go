//go:build windows

package diskcrypt

import (
	"context"
	"path/filepath"
	"strings"
)

// detect reads the BitLocker protection of the data folder's drive from the
// shell's System.Volume.BitLockerProtection property, which, unlike
// manage-bde, needs no administrator. Device Encryption shows there too.
func detect(ctx context.Context, path string) Status {
	drive := filepath.VolumeName(path)
	if drive == "" {
		drive = "C:"
	}
	script := "(New-Object -ComObject Shell.Application).NameSpace('" + strings.ReplaceAll(drive, "'", "") + "\\').Self.ExtendedProperty('System.Volume.BitLockerProtection')"
	out, err := run(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	if err != nil {
		return Status{State: Unknown}
	}
	return parseBitLocker(out)
}
