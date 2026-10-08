package diskcrypt

import "strings"

// parseBitLocker reads System.Volume.BitLockerProtection: 1 and 5 are
// protected, 3 is encrypting, 2 and 6 are off; anything else, such as an
// empty answer on a drive BitLocker doesn't cover, is unknown.
func parseBitLocker(out string) Status {
	switch strings.TrimSpace(out) {
	case "1", "5":
		return Status{State: On, Method: "BitLocker"}
	case "3":
		return Status{State: Encrypting, Method: "BitLocker"}
	case "2", "6":
		return Status{State: Off, Method: "BitLocker"}
	}
	return Status{State: Unknown}
}

// parseLsblk reads the TYPE column of lsblk -s: a crypt layer anywhere
// under the device is LUKS. A plain disk or partition is off; a network or
// virtual filesystem lsblk can't place is unknown.
func parseLsblk(out string) Status {
	types := strings.Fields(out)
	if len(types) == 0 {
		return Status{State: Unknown}
	}
	for _, t := range types {
		if t == "crypt" {
			return Status{State: On, Method: "LUKS"}
		}
	}
	for _, t := range types {
		if t == "disk" || t == "part" {
			return Status{State: Off, Method: "LUKS"}
		}
	}
	return Status{State: Unknown}
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return strings.TrimSpace(line)
}

// parseFileVault reads fdesetup status.
func parseFileVault(out string) Status {
	switch {
	case strings.Contains(out, "FileVault is On"):
		if strings.Contains(out, "Encryption in progress") {
			return Status{State: Encrypting, Method: "FileVault"}
		}
		return Status{State: On, Method: "FileVault"}
	case strings.Contains(out, "FileVault is Off"):
		if strings.Contains(out, "Encryption in progress") {
			return Status{State: Encrypting, Method: "FileVault"}
		}
		return Status{State: Off, Method: "FileVault"}
	}
	return Status{State: Unknown}
}
