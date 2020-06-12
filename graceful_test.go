package shutdown

import (
	"syscall"
	"testing"
	"time"
)

func TestInterrupt(t *testing.T) {
	signals := []syscall.Signal{
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	}

	for _, s := range signals {
		done := false
		go func() {
			// interrupt
			time.Sleep(10 * time.Millisecond)
			syscall.Kill(syscall.Getpid(), s)
		}()

		GracefulStop(func() {
			done = true
		})
		if !done {
			t.Fatal("Error: syscall.SIGHUP not handled")
		}
	}
}

func TestKill(t *testing.T) {
	signals := []syscall.Signal{
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	}

	for _, s := range signals {
		done := false
		go func() {
			// interrupt
			time.Sleep(10 * time.Millisecond)
			syscall.Kill(syscall.Getpid(), s)
			// kill
			time.Sleep(10 * time.Millisecond)
			syscall.Kill(syscall.Getpid(), s)
		}()

		GracefulStop(func() {
			time.Sleep(50 * time.Millisecond)
			done = true
		})
		if done {
			t.Fatal("Error: syscall.SIGHUP not killed before")
		}
	}
}
