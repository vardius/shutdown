//go:build unix

package shutdown

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"testing"
	"time"
)

// These tests send real signals to the test binary. Each sends one only while something listens for it, mostly
// GracefulStop once it has registered, so that no signal meets Go's default, which would end the binary.

// realSignals lets GracefulStop register for the process's signals, and hands out each channel it registers
// once it has.
func realSignals(t *testing.T) <-chan chan<- os.Signal {
	registered := make(chan chan<- os.Signal, 1)
	prev := notify
	notify = func(c chan<- os.Signal) {
		prev(c)
		registered <- c
	}
	t.Cleanup(func() { notify = prev })
	return registered
}

func kill(t *testing.T, sig syscall.Signal) {
	t.Helper()
	if err := syscall.Kill(os.Getpid(), sig); err != nil {
		t.Fatalf("send %v: %v", sig, err)
	}
}

func TestRealSignalsStartTheShutdown(t *testing.T) {
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP} {
		t.Run(sig.String(), func(t *testing.T) {
			registered := realSignals(t)
			stopped := make(chan struct{})
			done := make(chan error, 1)
			go func() {
				done <- GracefulStop(t.Context(), time.Minute, func(context.Context) error {
					close(stopped)
					return nil
				})
			}()

			<-registered
			kill(t, sig)
			if err := <-done; err != nil {
				t.Errorf("GracefulStop() = %v, want nil", err)
			}
			select {
			case <-stopped:
			default:
				t.Error("the stop didn't run")
			}
		})
	}
}

func TestRealSecondSignalEndsTheProcess(t *testing.T) {
	registered := realSignals(t)
	exits := fakeExit(t)
	stopping, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- GracefulStop(t.Context(), time.Minute, func(context.Context) error {
			close(stopping)
			<-release // a stop that hangs
			return nil
		})
	}()

	<-registered
	kill(t, syscall.SIGTERM)
	<-stopping
	kill(t, syscall.SIGINT)
	if code := <-exits; code != 1 {
		t.Errorf("exit(%d) on the second signal, want exit(1)", code)
	}

	close(release)
	<-done
}

func TestStopsListeningBeforeReturning(t *testing.T) {
	// Keeps the SIGTERM below from ending the test binary, whoever else listens for it.
	mine := make(chan os.Signal, 1)
	signal.Notify(mine, syscall.SIGTERM)

	registered := realSignals(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := GracefulStop(ctx, time.Minute); err != nil {
		t.Fatalf("GracefulStop() = %v, want nil", err)
	}
	theirs := <-registered

	kill(t, syscall.SIGTERM)
	<-mine
	signal.Stop(mine) // returns once the signal has reached every channel it is going to
	if len(theirs) != 0 {
		t.Error("GracefulStop's channel got a signal after GracefulStop returned")
	}
}

func TestIgnoredSignalsStayIgnored(t *testing.T) {
	signal.Ignore(syscall.SIGHUP) // as under nohup
	t.Cleanup(func() {
		// Notify and Stop make signal.Ignored report false again for the tests after this one, which listen for
		// SIGHUP themselves. Nothing restores the default: SIGHUP stays ignored while nothing listens for it.
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGHUP)
		signal.Stop(c)
	})

	registered := realSignals(t)
	done := make(chan error, 1)
	go func() { done <- GracefulStop(t.Context(), time.Minute) }()
	<-registered

	if !signal.Ignored(syscall.SIGHUP) {
		t.Error("GracefulStop turned SIGHUP back on")
	}
	kill(t, syscall.SIGTERM) // still listened for
	if err := <-done; err != nil {
		t.Errorf("GracefulStop() = %v, want nil", err)
	}
}

// TestSIGQUITIsLeftAlone runs itself in a child process, sends the child SIGQUIT while it waits in GracefulStop,
// and expects Go's default for it: a dump of every goroutine, and exit status 2.
func TestSIGQUITIsLeftAlone(t *testing.T) {
	const child = "SHUTDOWN_TEST_SIGQUIT_CHILD"
	if os.Getenv(child) == "1" {
		registered := realSignals(t)
		go func() {
			<-registered
			fmt.Println("listening")
		}()
		_ = GracefulStop(context.Background(), time.Second)
		return // only if SIGQUIT started a shutdown: the parent then fails
	}

	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestSIGQUITIsLeftAlone$")
	cmd.Env = append(os.Environ(), child+"=1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	lines := bufio.NewScanner(stdout)
	for lines.Scan() && lines.Text() != "listening" {
	}
	if err := cmd.Process.Signal(syscall.SIGQUIT); err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, stdout)
	err = cmd.Wait()

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 2 {
		t.Errorf("child ended with %v, want exit status 2", err)
	}
	if !strings.Contains(stderr.String(), "SIGQUIT: quit") {
		t.Errorf("child's stderr has no goroutine dump:\n%s", stderr.String())
	}
}
