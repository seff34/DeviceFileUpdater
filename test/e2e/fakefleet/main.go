//go:build !windows

// Command fakefleet serves the web UI next to a fleet of fake Telnet devices
// for the Playwright end-to-end test. Each device runs commands with the local
// /bin/sh inside its own temp dir: "/devroot" in a command is rewritten to that
// dir, so the test can check what the tool wrote. One extra device points at a
// closed port and is always unreachable.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"devupdater/internal/testutil"
	"devupdater/internal/web"
)

type device struct {
	Host     string `json:"host"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func main() {
	port := flag.Int("port", 8799, "web UI port")
	n := flag.Int("devices", 3, "number of healthy fake devices")
	info := flag.String("info", "fleet.json", "where to write the fleet description")
	flag.Parse()

	root, err := os.MkdirTemp("", "devupdater-e2e-")
	check(err)
	var devs []device
	for i := 0; i < *n; i++ {
		dir := filepath.Join(root, fmt.Sprintf("dev-%d", i))
		check(os.MkdirAll(dir, 0o755))
		pass := fmt.Sprintf("e2e-pass-%d", i)
		ln, err := testutil.ListenFakeTelnet("127.0.0.1:0", "root", pass, func(cmd string) (string, int) {
			out, code, err := testutil.LocalShell{}.Exec(context.Background(), strings.ReplaceAll(cmd, "/devroot", dir))
			if err != nil {
				return err.Error(), 127
			}
			return out, code
		})
		check(err)
		devs = append(devs, device{ln.Addr().String(), "root", pass})
	}
	closed, err := net.Listen("tcp", "127.0.0.1:0")
	check(err)
	down := closed.Addr().String()
	closed.Close()
	devs = append(devs, device{down, "root", "e2e-pass-down"})

	parent := filepath.Join(root, "workspaces")
	check(os.MkdirAll(parent, 0o755))
	srv, err := web.New(web.Options{Addr: fmt.Sprintf("127.0.0.1:%d", *port), ConfigDir: filepath.Join(root, "config"), Token: "e2e-token"})
	check(err)
	url, err := srv.Listen()
	check(err)
	b, _ := json.MarshalIndent(map[string]any{
		"url": url, "token": srv.Token(), "root": root, "parent": parent, "devices": devs, "down": down,
	}, "", "  ")
	check(os.WriteFile(*info, b, 0o600))
	log.Printf("fakefleet: %d devices, UI on 127.0.0.1:%d", len(devs), *port)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	err = srv.Serve(ctx)
	os.RemoveAll(root)
	check(err)
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
