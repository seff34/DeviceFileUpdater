package workspace

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestParseDevices(t *testing.T) {
	in := "ip,username,password\n192.168.1.10,root,s1\n192.168.1.11:2222,admin,\"s,2\"\n"
	ds, err := ParseDevices(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) != 2 || ds[1].Password != "s,2" || ds[1].Host != "192.168.1.11:2222" {
		t.Fatalf("got %+v", ds)
	}
	if ds[0].Addr(22) != "192.168.1.10:22" || ds[1].Addr(22) != "192.168.1.11:2222" {
		t.Fatalf("addr: %s %s", ds[0].Addr(22), ds[1].Addr(22))
	}
	if ds[1].HostOnly() != "192.168.1.11" {
		t.Fatalf("hostonly %s", ds[1].HostOnly())
	}
}

func TestParseDevicesErrors(t *testing.T) {
	bad := []string{
		"",
		"host,user,pass\n1.1.1.1,a,b\n", // wrong header
		"ip,username,password\n1.1.1.1,a,b\n1.1.1.1,c,d\n", // duplicate
		"ip,username,password\n,a,b\n",                     // empty ip
		"ip,username,password\n1.1.1.1,,b\n",               // empty user
	}
	for _, in := range bad {
		if _, err := ParseDevices(strings.NewReader(in)); err == nil {
			t.Errorf("expected error for %q", in)
		}
	}
}

func TestSaveLoadDevicesRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "devices.csv")
	want := []Device{{Host: "10.0.0.1", Username: "root", Password: `p"w,1`}}
	if err := SaveDevices(p, want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadDevices(p)
	if err != nil || len(got) != 1 || got[0] != want[0] {
		t.Fatalf("got %+v err %v", got, err)
	}
}

func TestSaveDevicesFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping permission test on windows")
	}
	p := filepath.Join(t.TempDir(), "devices.csv")
	want := []Device{{Host: "10.0.0.1", Username: "root", Password: "secret"}}
	if err := SaveDevices(p, want); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected perm 0o600, got %o", info.Mode().Perm())
	}
}

func TestParseDevicesWithBOM(t *testing.T) {
	in := "\xef\xbb\xbfip,username,password\n192.168.1.10,root,s1\n"
	ds, err := ParseDevices(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) != 1 || ds[0].Host != "192.168.1.10" {
		t.Fatalf("got %+v", ds)
	}
}
