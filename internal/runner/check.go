package runner

import (
	"context"
	"errors"
	"net"
	"sort"
	"sync"

	"devupdater/internal/probe"
	"devupdater/internal/syncer"
	"devupdater/internal/transport"
	"devupdater/internal/upload"
	"devupdater/internal/workspace"
)

// CheckResult is one device's connection test: what a real run would use.
type CheckResult struct {
	Host          string   `json:"host"`
	OK            bool     `json:"ok"`
	Protocol      string   `json:"protocol,omitempty"`
	Tools         []string `json:"tools,omitempty"`
	UploadMethods []string `json:"upload_methods,omitempty"`
	HashMethod    string   `json:"hash_method,omitempty"`
	Error         string   `json:"error,omitempty"`
}

// Check connects, probes and reports the upload/hash methods a run would pick.
// It writes nothing to the device. Error text is Turkish (shown in the UI)
// followed by the redacted technical cause.
func Check(ctx context.Context, job Job, d workspace.Device) (r CheckResult) {
	if job.Dial == nil {
		job.Dial = transport.Dial
	}
	if job.Probe == nil {
		job.Probe = probe.Probe
	}
	r.Host = d.Host
	st := job.Settings
	cmdTimeout := secs(st.CommandTimeoutSec, defaultCommandTimeout)
	opt := transport.Options{
		ConnectTimeout: secs(st.ConnectTimeoutSec, defaultConnectTimeout),
		CommandTimeout: cmdTimeout,
		StrictHostKey:  st.StrictHostKey,
		KnownHostsPath: job.KnownHostsPath,
	}
	s, err := job.Dial(ctx, d, opt)
	if err == nil && s == nil {
		err = errors.New("no session")
	}
	if err != nil {
		msg := "Cihaza ulaşılamadı"
		switch {
		case ctx.Err() != nil:
			msg = "İptal edildi"
		case errors.Is(err, transport.ErrHostKey):
			msg = "Host key uyuşmuyor"
		case errors.Is(err, transport.ErrAuth):
			msg = "Kullanıcı adı veya şifre hatalı"
		}
		r.Error = redact(msg+": "+err.Error(), d.Password)
		return r
	}
	defer s.Close()
	r.Protocol = s.Protocol()

	ftpHost := d.HostOnly()
	if _, _, err := net.SplitHostPort(d.Host); err == nil {
		ftpHost = ""
	}
	caps, err := job.Probe(ctx, s, ftpHost, opt.ConnectTimeout, cmdTimeout)
	if err != nil {
		r.Error = redact("Yetenek tespiti başarısız: "+err.Error(), d.Password)
		return r
	}
	r.Tools = caps.List()
	sort.Strings(r.Tools)
	for _, u := range upload.Select(s, caps, upload.FTPCreds{Host: d.HostOnly(), User: d.Username, Pass: d.Password}, cmdTimeout) {
		r.UploadMethods = append(r.UploadMethods, u.Name())
	}
	r.HashMethod = "none"
	if h := syncer.SelectHasher(s, caps, cmdTimeout); h != nil {
		r.HashMethod = h.Name()
	}
	r.OK = true
	return r
}

// CheckAll runs Check for job.Devices with job.Settings.Parallel concurrency.
func CheckAll(ctx context.Context, job Job, emit func(CheckResult)) {
	var mu sync.Mutex
	sem := make(chan struct{}, max(1, job.Settings.Parallel))
	var wg sync.WaitGroup
	for _, d := range job.Devices {
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				mu.Lock()
				emit(CheckResult{Host: d.Host, Error: "İptal edildi"})
				mu.Unlock()
				return
			}
			r := Check(ctx, job, d)
			mu.Lock()
			emit(r)
			mu.Unlock()
		}()
	}
	wg.Wait()
}
