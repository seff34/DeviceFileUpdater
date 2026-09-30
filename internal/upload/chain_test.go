package upload

import (
	"context"
	"errors"
	"sync"
	"testing"
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
	c := NewChain([]Uploader{a, b})
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
	c := NewChain([]Uploader{&fakeUp{name: "a", fail: true}, &fakeUp{name: "b", fail: true}})
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
	c := NewChain([]Uploader{a, &fakeUp{name: "b"}})
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
	c := NewChain([]Uploader{&fakeUp{name: "a", fail: true}, &fakeUp{name: "b"}})
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
	c := NewChain([]Uploader{cancelUp{cancel}, &fakeUp{name: "b"}})
	if _, err := c.Upload(ctx, nil, "/t"); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
	if c.Primary() != "c" {
		t.Fatalf("primary changed to %q", c.Primary())
	}
}
