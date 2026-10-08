package diskcrypt

import (
	"context"
	"testing"
)

func TestParsers(t *testing.T) {
	for _, tc := range []struct {
		name string
		got  Status
		want Status
	}{
		{"filevault on", parseFileVault("FileVault is On."), Status{On, "FileVault"}},
		{"filevault off", parseFileVault("FileVault is Off."), Status{Off, "FileVault"}},
		{"filevault encrypting", parseFileVault("FileVault is On.\nEncryption in progress: Percent completed = 42.0"), Status{Encrypting, "FileVault"}},
		{"filevault odd", parseFileVault("something else"), Status{State: Unknown}},
		{"bitlocker on", parseBitLocker("1"), Status{On, "BitLocker"}},
		{"bitlocker off", parseBitLocker("2\r\n"), Status{Off, "BitLocker"}},
		{"bitlocker encrypting", parseBitLocker("3"), Status{Encrypting, "BitLocker"}},
		{"bitlocker none", parseBitLocker(""), Status{State: Unknown}},
		{"luks", parseLsblk("part\ncrypt\nlvm\n"), Status{On, "LUKS"}},
		{"luks under lvm", parseLsblk("lvm\ncrypt\npart\ndisk"), Status{On, "LUKS"}},
		{"plain", parseLsblk("part\ndisk"), Status{Off, "LUKS"}},
		{"overlay", parseLsblk("loop"), Status{State: Unknown}},
		{"nothing", parseLsblk(""), Status{State: Unknown}},
	} {
		if tc.got != tc.want {
			t.Errorf("%s: %+v, want %+v", tc.name, tc.got, tc.want)
		}
	}
	if firstLine("/dev/mapper/root\n/dev/sda2") != "/dev/mapper/root" {
		t.Error("firstLine")
	}
}

// An answer is remembered, so the settings page doesn't run a command each
// time it opens.
func TestDetectIsCached(t *testing.T) {
	calls := 0
	old := run
	run = func(context.Context, string, ...string) (string, error) { calls++; return "", nil }
	t.Cleanup(func() { run = old })
	cache.path = ""
	first := Detect(context.Background(), t.TempDir()+"/x")
	again := Detect(context.Background(), cache.path)
	if first.State == "" || again != first || calls == 0 {
		t.Fatalf("first %+v again %+v calls %d", first, again, calls)
	}
	n := calls
	Detect(context.Background(), cache.path)
	if calls != n {
		t.Fatal("a cached answer ran the command again")
	}
}
