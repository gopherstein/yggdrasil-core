//go:build !darwin && !windows && !linux

package diskcrypt

import "context"

func detect(context.Context, string) Status { return Status{State: Unknown} }
