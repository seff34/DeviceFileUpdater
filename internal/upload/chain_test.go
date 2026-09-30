package upload

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeUp struct {
	name  string
	fail  bool
	mu    sync.Mutex
	calls int
}

func (f *fakeUp) Name() string { return f.name }
func (f *fakeUp) Upload(context.Context, []byte, string) error {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	if f.fail {
		return errors.New(f.name + " broken")
	}
	return nil
}

func TestChainFallsBackAndRemembers(t *testing.T) {
	a, b := &fakeUp{name: "sftp", fail: true}, &fakeUp{name: "shell-printf"}
	c := NewChain([]Uploader{a, b}, time.Second)
	if c.Primary() != "sftp" {
		t.Fatal(c.Primary())
	}
	for i := 0; i < 2; i++ {
		m, err := c.Upload(context.Background(), []byte("x"), "/t")
		if err != nil || m != "shell-printf" {
			t.Fatalf("m=%s err=%v", m, err)
		}
	}
	if a.calls != 1 {
		t.Fatalf("broken uploader retried: %d calls", a.calls)
	}
	if c.Primary() != "shell-printf" {
		t.Fatal(c.Primary())
	}
}

func TestChainAllFail(t *testing.T) {
	c := NewChain([]Uploader{&fakeUp{name: "a", fail: true}, &fakeUp{name: "b", fail: true}}, time.Second)
	if _, err := c.Upload(context.Background(), nil, "/t"); err == nil {
		t.Fatal("expected error")
	}
	if c.Primary() != "" {
		t.Fatal("all broken: primary should be empty")
	}
	if _, err := c.Upload(context.Background(), nil, "/t"); err == nil {
		t.Fatal("expected error when every method is already broken")
	}
}

func TestChainStopsOnCancel(t *testing.T) {
	a := &fakeUp{name: "a", fail: true}
	c := NewChain([]Uploader{a, &fakeUp{name: "b"}}, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Upload(ctx, nil, "/t"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancel, got %v", err)
	}
	if a.calls != 0 {
		t.Fatal("uploader called after cancel")
	}
}

func TestChainConcurrentUse(t *testing.T) {
	c := NewChain([]Uploader{&fakeUp{name: "a", fail: true}, &fakeUp{name: "b"}}, time.Second)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Upload(context.Background(), nil, "/t")
			c.Primary()
		}()
	}
	wg.Wait()
}

type cancelUp struct{ cancel context.CancelFunc }

func (cancelUp) Name() string { return "c" }
func (u cancelUp) Upload(context.Context, []byte, string) error {
	u.cancel()
	return errors.New("aborted")
}

func TestChainCancelDuringUploadNotMarkedBroken(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	c := NewChain([]Uploader{cancelUp{cancel}, &fakeUp{name: "b"}}, time.Second)
	if _, err := c.Upload(ctx, nil, "/t"); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
	if c.Primary() != "c" {
		t.Fatalf("primary changed to %q", c.Primary())
	}
}

func TestTransferTimeoutScalesWithSize(t *testing.T) {
	if got := transferTimeout(time.Second, 0); got != time.Second {
		t.Fatalf("empty: %v", got)
	}
	if got := transferTimeout(time.Second, 64<<10); got != 3*time.Second {
		t.Fatalf("64 KiB: %v", got)
	}
	if got := transferTimeout(0, 0); got != defaultCmdTimeout {
		t.Fatalf("zero cmd timeout: %v", got)
	}
}

// stallUp blocks until its ctx ends, like a transfer to a wedged device.
type stallUp struct{}

func (stallUp) Name() string { return "sftp" }
func (stallUp) Upload(ctx context.Context, _ []byte, _ string) error {
	<-ctx.Done()
	return ctx.Err()
}

// A stalled network uploader times out on its own budget, is marked broken,
// and the next method still runs.
func TestChainStalledUploaderFallsBack(t *testing.T) {
	b := &fakeUp{name: "shell-printf"}
	c := NewChain([]Uploader{stallUp{}, b}, 100*time.Millisecond)
	start := time.Now()
	m, err := c.Upload(context.Background(), []byte("x"), "/t")
	if err != nil || m != "shell-printf" || b.calls != 1 {
		t.Fatalf("m=%s err=%v calls=%d", m, err, b.calls)
	}
	if el := time.Since(start); el > 2*time.Second {
		t.Fatalf("took %v", el)
	}
	if c.Primary() != "shell-printf" {
		t.Fatalf("stalled uploader not marked broken: primary %s", c.Primary())
	}
}

func TestShellUploadersHaveNoWholeUploadDeadline(t *testing.T) {
	for _, u := range []Uploader{NewShellBase64(nil, 0), NewShellPrintf(nil, 0)} {
		if _, ok := u.(perCommand); !ok {
			t.Errorf("%s must be bounded per command only", u.Name())
		}
	}
	for _, u := range []Uploader{NewSFTP(nil), NewSCP(nil), NewFTP(nil, FTPCreds{}, 0)} {
		if _, ok := u.(perCommand); ok {
			t.Errorf("%s must get a transfer deadline", u.Name())
		}
	}
}
