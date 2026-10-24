package shutdown

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// GracefulStop blocks until the process gets SIGINT, SIGTERM or SIGHUP, or ctx is done. Then it calls each stop,
// one after another in the order given, and returns their errors joined.
//
// The stops share one context, done timeout after the shutdown starts; with a timeout of 0 or less it is done
// from the start. It carries ctx's values but not its cancellation. Every stop is called, even once that context
// is done, so each should return early when it is.
//
// The second signal GracefulStop receives ends the process at once, with exit status 1: GracefulStop doesn't return
// and deferred calls don't run. It is the way out when a stop hangs. Signals the program ignores stay ignored,
// and GracefulStop stops listening for signals before it returns.
//
// GracefulStop panics if a stop is nil, at the call rather than at the shutdown.
func GracefulStop(ctx context.Context, timeout time.Duration, stops ...func(context.Context) error) error {
	for _, stop := range stops {
		if stop == nil {
			panic("shutdown: nil stop")
		}
	}

	// Room for both signals that matter, even if they arrive together.
	signals := make(chan os.Signal, 2)
	notify(signals)
	defer signal.Stop(signals)

	started := make(chan struct{})
	returned := make(chan struct{})
	watched := make(chan struct{})
	go func() {
		watch(signals, started, returned)
		close(watched)
	}()
	// Waits for watch, so GracefulStop can't return while a second signal is ending the process.
	defer func() {
		close(returned)
		<-watched
	}()

	select {
	case <-started:
	case <-ctx.Done():
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()

	errs := make([]error, len(stops))
	for i, stop := range stops {
		errs[i] = stop(ctx)
	}
	return errors.Join(errs...)
}

// watch closes started on the first signal and ends the process on the second, until returned is closed.
//
// It counts signals, not shutdowns: when ctx started the shutdown, the signal that cancelled ctx (through
// signal.NotifyContext, say) may also be on its way here, and that one signal must not end the process.
func watch(signals <-chan os.Signal, started, returned chan struct{}) {
	select {
	case <-signals:
		close(started)
	case <-returned:
		return
	}

	select {
	case <-signals:
		exit(1)
	case <-returned:
	}
}

// notify relays the shutdown signals to c. It and exit are variables so that tests can stand in for the process.
var notify = func(c chan<- os.Signal) {
	for _, sig := range []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP} {
		// Notify would turn an ignored signal back on: SIGHUP under nohup, for one.
		if !signal.Ignored(sig) {
			signal.Notify(c, sig)
		}
	}
}

var exit = os.Exit
