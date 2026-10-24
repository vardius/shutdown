package shutdown

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"slices"
	"syscall"
	"testing"
	"testing/synctest"
	"time"
)

var (
	errA = errors.New("a failed")
	errB = errors.New("b failed")
)

func init() {
	// No test may end the test binary. A test that expects an exit swaps this for fakeExit.
	exit = func(code int) { panic(fmt.Sprintf("exit(%d) without fakeExit", code)) }
}

// fakeSignals stands in for the process's signals. Each channel GracefulStop registers comes out of the returned
// one, and the test sends it what the process would.
func fakeSignals(t *testing.T) <-chan chan<- os.Signal {
	registered := make(chan chan<- os.Signal, 1)
	prev := notify
	notify = func(c chan<- os.Signal) { registered <- c }
	t.Cleanup(func() { notify = prev })
	return registered
}

// fakeExit records the code GracefulStop would end the process with, instead of ending it.
func fakeExit(t *testing.T) <-chan int {
	exits := make(chan int, 2) // room for one exit too many, so that a test fails rather than deadlocks
	prev := exit
	exit = func(code int) { exits <- code }
	t.Cleanup(func() { exit = prev })
	return exits
}

// record returns a stop that appends name to calls and returns err.
func record(calls *[]string, name string, err error) func(context.Context) error {
	return func(context.Context) error {
		*calls = append(*calls, name)
		return err
	}
}

func TestNilStopPanicsAtTheCall(t *testing.T) {
	prev := notify
	notify = func(chan<- os.Signal) { t.Error("GracefulStop listened for signals") }
	t.Cleanup(func() { notify = prev })
	defer func() {
		if r := recover(); r != "shutdown: nil stop" {
			t.Errorf("panic = %v, want %q", r, "shutdown: nil stop")
		}
	}()
	_ = GracefulStop(t.Context(), time.Second, record(new([]string), "a", nil), nil)
}

// Regression: GracefulStop returned, and main went on, while a second signal that came as the last stop returned
// was ending the process.
func TestNoReturnWhileTheSecondSignalEndsTheProcess(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		registered := fakeSignals(t)
		exiting, release := make(chan struct{}), make(chan struct{})
		prev := exit
		exit = func(int) {
			close(exiting)
			<-release // os.Exit doesn't return; this one waits to be let go
		}
		t.Cleanup(func() { exit = prev })

		var signals chan<- os.Signal
		done := make(chan error, 1)
		go func() {
			done <- GracefulStop(t.Context(), time.Minute, func(context.Context) error {
				signals <- syscall.SIGINT
				<-exiting
				return nil
			})
		}()
		signals = <-registered
		signals <- syscall.SIGTERM

		synctest.Wait()
		select {
		case err := <-done:
			t.Fatalf("GracefulStop() = %v while the process was exiting, want no return", err)
		default:
		}
		close(release)
		<-done
	})
}

// Two signals can arrive before anything reads them. os/signal drops a signal its channel has no room for,
// so the channel must hold both, or the second one never ends the process.
func TestTwoSignalsAtOnce(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		prev := notify
		notify = func(c chan<- os.Signal) {
			for _, sig := range []os.Signal{syscall.SIGTERM, os.Interrupt} {
				select {
				case c <- sig:
				default: // as os/signal does
				}
			}
		}
		t.Cleanup(func() { notify = prev })
		exits := fakeExit(t)

		release := make(chan struct{})
		done := make(chan error, 1)
		go func() {
			done <- GracefulStop(t.Context(), time.Minute, func(context.Context) error {
				<-release // a stop that hangs
				return nil
			})
		}()
		if code := <-exits; code != 1 {
			t.Errorf("exit(%d), want exit(1)", code)
		}
		close(release)
		<-done
	})
}

func TestGracefulStopOnSignal(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var logged bytes.Buffer
		log.SetOutput(&logged)
		t.Cleanup(func() { log.SetOutput(os.Stderr) })

		registered := fakeSignals(t)
		var calls []string
		done := make(chan error, 1)
		go func() {
			done <- GracefulStop(t.Context(), time.Second, record(&calls, "a", nil), record(&calls, "b", nil))
		}()
		signals := <-registered

		time.Sleep(24 * time.Hour)
		if len(calls) != 0 {
			t.Errorf("stops ran before a signal: %v", calls)
		}

		signals <- syscall.SIGTERM
		if err := <-done; err != nil {
			t.Errorf("GracefulStop() = %v, want nil", err)
		}
		if want := []string{"a", "b"}; !slices.Equal(calls, want) {
			t.Errorf("stops ran %v, want %v", calls, want)
		}
		if logged.Len() != 0 {
			t.Errorf("GracefulStop logged %q, want nothing", logged.String())
		}
	})
}

func TestGracefulStopWhenCtxIsDone(t *testing.T) {
	tests := []struct {
		name    string
		ctx     func(context.Context) (context.Context, context.CancelFunc)
		elapsed time.Duration
	}{
		{
			name: "cancelled",
			ctx: func(parent context.Context) (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(parent)
				time.AfterFunc(time.Hour, cancel)
				return ctx, cancel
			},
			elapsed: time.Hour,
		},
		{
			name: "deadline",
			ctx: func(parent context.Context) (context.Context, context.CancelFunc) {
				return context.WithTimeout(parent, 2*time.Hour)
			},
			elapsed: 2 * time.Hour,
		},
		{
			name: "already done",
			ctx: func(parent context.Context) (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(parent)
				cancel()
				return ctx, cancel
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				fakeSignals(t)
				ctx, cancel := tt.ctx(t.Context())
				defer cancel()

				var calls []string
				start := time.Now()
				err := GracefulStop(ctx, time.Second, record(&calls, "a", nil))

				if err != nil {
					t.Errorf("GracefulStop() = %v, want nil", err)
				}
				if want := []string{"a"}; !slices.Equal(calls, want) {
					t.Errorf("stops ran %v, want %v", calls, want)
				}
				if elapsed := time.Since(start); elapsed != tt.elapsed {
					t.Errorf("GracefulStop() returned after %v, want %v", elapsed, tt.elapsed)
				}
			})
		})
	}
}

func TestGracefulStopWithNoStops(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		registered := fakeSignals(t)
		done := make(chan error, 1)
		go func() { done <- GracefulStop(t.Context(), time.Second) }()

		(<-registered) <- syscall.SIGINT
		if err := <-done; err != nil {
			t.Errorf("GracefulStop() = %v, want nil", err)
		}
	})
}

func TestStopsRunOneAfterAnother(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fakeSignals(t)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		start := time.Now()
		var began []time.Duration
		stop := func(d time.Duration) func(context.Context) error {
			return func(context.Context) error {
				began = append(began, time.Since(start))
				time.Sleep(d)
				return nil
			}
		}
		_ = GracefulStop(ctx, time.Minute, stop(time.Second), stop(2*time.Second), stop(0))

		// Each begins when the one before it returns.
		if want := []time.Duration{0, time.Second, 3 * time.Second}; !slices.Equal(began, want) {
			t.Errorf("stops began at %v, want %v", began, want)
		}
	})
}

func TestStopsShareOneDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		registered := fakeSignals(t)
		done := make(chan error, 1)

		var deadlines []time.Time
		var errsAtCall []error
		stop := func(wait func(ctx context.Context) error) func(context.Context) error {
			return func(ctx context.Context) error {
				deadline, _ := ctx.Deadline()
				deadlines = append(deadlines, deadline)
				errsAtCall = append(errsAtCall, ctx.Err())
				return wait(ctx)
			}
		}
		go func() {
			done <- GracefulStop(t.Context(), 10*time.Second,
				stop(func(context.Context) error { time.Sleep(4 * time.Second); return nil }),
				stop(func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }),
				stop(func(context.Context) error { return nil }),
			)
		}()
		signals := <-registered
		time.Sleep(time.Hour) // the deadline counts from the signal, not from the call
		start := time.Now()
		signals <- syscall.SIGTERM
		err := <-done

		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("GracefulStop() = %v, want %v from the stop that waited", err, context.DeadlineExceeded)
		}
		if elapsed := time.Since(start); elapsed != 10*time.Second {
			t.Errorf("GracefulStop() returned %v after the signal, want 10s", elapsed)
		}
		want := start.Add(10 * time.Second)
		if !slices.EqualFunc(deadlines, []time.Time{want, want, want}, time.Time.Equal) {
			t.Errorf("stops' deadlines = %v, want %v for every one", deadlines, want)
		}
		// The last stop is still called, with its context already done.
		if want := []error{nil, nil, context.DeadlineExceeded}; !slices.Equal(errsAtCall, want) {
			t.Errorf("stops' ctx.Err() when called = %v, want %v", errsAtCall, want)
		}
	})
}

func TestTimeoutOfZeroOrLess(t *testing.T) {
	for _, timeout := range []time.Duration{0, -time.Second} {
		t.Run(timeout.String(), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				fakeSignals(t)
				ctx, cancel := context.WithCancel(t.Context())
				cancel()

				var errAtCall error
				_ = GracefulStop(ctx, timeout, func(ctx context.Context) error {
					errAtCall = ctx.Err()
					return nil
				})
				if errAtCall != context.DeadlineExceeded {
					t.Errorf("stop's ctx.Err() = %v, want %v: no time to stop", errAtCall, context.DeadlineExceeded)
				}
			})
		})
	}
}

func TestErrorsJoinedInOrder(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fakeSignals(t)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		var calls []string
		err := GracefulStop(ctx, time.Second, record(&calls, "a", errA), record(&calls, "ok", nil), record(&calls, "b", errB))

		if !errors.Is(err, errA) || !errors.Is(err, errB) {
			t.Errorf("GracefulStop() = %v, want both %v and %v", err, errA, errB)
		}
		if want := "a failed\nb failed"; err == nil || err.Error() != want {
			t.Errorf("GracefulStop() = %q, want %q", err, want)
		}
		if want := []string{"a", "ok", "b"}; !slices.Equal(calls, want) {
			t.Errorf("stops ran %v, want every one: %v", calls, want)
		}
	})
}

func TestStopsGetCtxValuesButNotItsCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fakeSignals(t)
		type key struct{}
		ctx, cancel := context.WithCancel(context.WithValue(t.Context(), key{}, "request-id"))
		cancel()

		var value any
		var errAtCall error
		_ = GracefulStop(ctx, time.Second, func(ctx context.Context) error {
			value, errAtCall = ctx.Value(key{}), ctx.Err()
			return nil
		})

		if value != "request-id" {
			t.Errorf("stop's ctx.Value() = %v, want %q", value, "request-id")
		}
		if errAtCall != nil {
			t.Errorf("stop's ctx.Err() = %v, want nil: ctx was cancelled, but the stops have their own deadline", errAtCall)
		}
	})
}

func TestSecondSignalEndsTheProcess(t *testing.T) {
	tests := []struct {
		name string
		// start starts the shutdown, and says how many signals it sent to do so.
		start func(signals chan<- os.Signal, cancel context.CancelFunc) (sent int)
	}{
		{
			name: "a signal started the shutdown",
			start: func(signals chan<- os.Signal, _ context.CancelFunc) int {
				signals <- syscall.SIGINT
				return 1
			},
		},
		{
			// As with signal.NotifyContext: one signal both cancels ctx and reaches GracefulStop.
			name: "the signal that cancelled ctx arrives after it",
			start: func(signals chan<- os.Signal, cancel context.CancelFunc) int {
				cancel()
				synctest.Wait()
				signals <- syscall.SIGINT
				return 1
			},
		},
		{
			name: "the signal that cancelled ctx arrives with it",
			start: func(signals chan<- os.Signal, cancel context.CancelFunc) int {
				cancel()
				signals <- syscall.SIGINT
				return 1
			},
		},
		{
			name: "ctx started it with no signal",
			start: func(_ chan<- os.Signal, cancel context.CancelFunc) int {
				cancel()
				return 0
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				registered := fakeSignals(t)
				exits := fakeExit(t)
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()

				stopping := make(chan struct{})
				done := make(chan error, 1)
				go func() {
					done <- GracefulStop(ctx, time.Hour, func(ctx context.Context) error {
						close(stopping)
						<-ctx.Done() // a stop that hangs until its deadline
						return ctx.Err()
					})
				}()
				signals := <-registered

				sent := tt.start(signals, cancel)
				<-stopping
				for ; sent < 1; sent++ {
					signals <- syscall.SIGHUP
				}
				synctest.Wait()
				if len(exits) != 0 {
					t.Errorf("exit(%d) after one signal, want none", <-exits)
				}

				start := time.Now()
				signals <- syscall.SIGTERM
				synctest.Wait()
				select {
				case code := <-exits:
					if code != 1 {
						t.Errorf("exit(%d) on the second signal, want exit(1)", code)
					}
				default:
					t.Error("no exit on the second signal")
				}
				if elapsed := time.Since(start); elapsed != 0 {
					t.Errorf("exit after %v, want at once", elapsed)
				}

				// The real exit never returns. The fake does, so let the stop reach its deadline.
				<-done
			})
		})
	}
}

func TestPanickingStop(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fakeSignals(t)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		var calls []string
		defer func() {
			if r := recover(); r != "boom" {
				t.Errorf("recovered %v, want the stop's panic", r)
			}
			if want := []string{"a"}; !slices.Equal(calls, want) {
				t.Errorf("stops ran %v, want %v", calls, want)
			}
			// synctest.Test fails if a goroutine GracefulStop started outlives it.
		}()
		_ = GracefulStop(ctx, time.Second, record(&calls, "a", nil), func(context.Context) error { panic("boom") }, record(&calls, "c", nil))
	})
}

func TestGracefulStopAgain(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		registered := fakeSignals(t)
		var calls []string
		for _, name := range []string{"first", "second"} {
			done := make(chan error, 1)
			go func() { done <- GracefulStop(t.Context(), time.Second, record(&calls, name, nil)) }()
			(<-registered) <- syscall.SIGTERM
			if err := <-done; err != nil {
				t.Errorf("GracefulStop() = %v, want nil", err)
			}
		}
		if want := []string{"first", "second"}; !slices.Equal(calls, want) {
			t.Errorf("stops ran %v, want %v", calls, want)
		}
	})
}
