package main

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"testing"
	"time"
)

// closeNotifier is a listener that closes closed when the server closes it, which it does as the shutdown begins.
type closeNotifier struct {
	net.Listener
	once   sync.Once
	closed chan struct{}
}

func (l *closeNotifier) Close() error {
	l.once.Do(func() { close(l.closed) })
	return l.Listener.Close()
}

func listen(t *testing.T) *closeNotifier {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	return &closeNotifier{Listener: ln, closed: make(chan struct{})}
}

type response struct {
	body string
	err  error
}

func get(url string) <-chan response {
	got := make(chan response, 1)
	go func() {
		resp, err := http.Get(url)
		if err != nil {
			got <- response{err: err}
			return
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		got <- response{string(body), err}
	}()
	return got
}

func TestServeDrainsOnSIGTERM(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a process can't send itself SIGTERM on Windows")
	}
	// The SIGTERM below may arrive before serve's GracefulStop has registered for it.
	// This context turns it into a shutdown all the same, and keeps it from ending the test binary.
	ctx, stop := signal.NotifyContext(t.Context(), syscall.SIGTERM)
	defer stop()

	ln := listen(t)
	inFlight := make(chan struct{})
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(inFlight)
		<-ln.closed // answer only once the shutdown has begun
		io.WriteString(w, "done")
	})
	served := make(chan error, 1)
	go func() { served <- serve(ctx, ln, h, time.Minute) }()

	got := get("http://" + ln.Addr().String())
	<-inFlight
	self, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if err := self.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}

	if r := <-got; r.body != "done" || r.err != nil {
		t.Errorf("request in flight got %q, %v, want %q, nil", r.body, r.err, "done")
	}
	if err := <-served; err != nil {
		t.Errorf("serve() = %v, want nil", err)
	}
	if resp, err := http.Get("http://" + ln.Addr().String()); err == nil {
		resp.Body.Close()
		t.Error("server still answers after shutting down")
	}
}

func TestServeCutsOffRequestsAfterTheGracePeriod(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	ln := listen(t)
	inFlight := make(chan struct{})
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(inFlight)
		<-r.Context().Done() // never answers on its own
	})
	served := make(chan error, 1)
	go func() { served <- serve(ctx, ln, h, 0) }() // no grace period at all

	got := get("http://" + ln.Addr().String())
	<-inFlight
	cancel()

	if err := <-served; !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("serve() = %v, want %v", err, context.DeadlineExceeded)
	}
	if r := <-got; r.err == nil {
		t.Errorf("request in flight got %q, want it cut off", r.body)
	}
}

func TestServeShutsDownWhenTheServerFails(t *testing.T) {
	ln := listen(t)
	ln.Close() // so the server fails at its first Accept

	if err := serve(t.Context(), ln, http.NotFoundHandler(), time.Minute); !errors.Is(err, net.ErrClosed) {
		t.Errorf("serve() = %v, want %v", err, net.ErrClosed)
	}
}
