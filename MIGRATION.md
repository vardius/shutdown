# Migrating from v1 to v2

v2 keeps `GracefulStop` and SIGINT, SIGTERM and SIGHUP, but it returns now. v1 ran one function and then called `os.Exit(0)`,
so nothing after it ran, deferred calls included. v2 runs your stop functions with a deadline, returns their errors,
and leaves the exit to you.

## 1. Update the import path

```sh
go get github.com/vardius/shutdown/v2
```

```go
import "github.com/vardius/shutdown/v2"
```

v2 needs Go 1.26 or newer.

## 2. Before and after

```go
// v1
stop := func() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}

go func() {
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Printf("HTTP server ListenAndServe: %v", err)
		os.Exit(1)
	}
}()

shutdown.GracefulStop(stop) // calls os.Exit(0) once stop returns
```

```go
// v2
go func() {
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}()

if err := shutdown.GracefulStop(context.Background(), 5*time.Second, srv.Shutdown); err != nil {
	log.Fatal(err)
}
```

The timeout moves out of your function and into the call: `GracefulStop` makes the context and hands it to every stop.

## 3. Every call

| v1 | v2 |
| --- | --- |
| `GracefulStop(stop func())` | `GracefulStop(ctx context.Context, timeout time.Duration, stops ...func(context.Context) error) error` |
| A stop that makes its own context: `func() { ctx, cancel := context.WithTimeout(…); … }` | A stop that takes one: `func(ctx context.Context) error`, such as `srv.Shutdown` |
| A stop with no context: `func() { db.Close() }` | `func(context.Context) error { return db.Close() }` |
| Everything in one function | One stop each, in the order they should run |
| Code after `GracefulStop` never runs | It runs once the stops return: exit with `log.Fatal(err)` or `os.Exit(1)` if `err` isn't nil |
| Only a signal starts the shutdown | A signal, or `ctx` done: pass `context.Background()` for signals alone |

## 4. Behaviour changes

- **GracefulStop returns.** v1 called `os.Exit(0)` once the stop returned, so code after it and deferred calls never ran,
  and the exit status was 0 even when the shutdown failed. v2 returns the stops' errors joined, or nil.
- **The stops get a deadline.** Their context is done `timeout` after the shutdown starts; with a timeout of 0 or less,
  it is done from the start. It carries `ctx`'s values but not its cancellation.
- **ctx starts the shutdown too.** When it's done, the stops run as if a signal had arrived.
- **SIGQUIT is no longer caught.** v1 treated it like the others. v2 leaves it to Go, so Ctrl+\ prints every goroutine's
  stack and exits with status 2.
- **Ignored signals stay ignored.** v1 turned SIGHUP back on for a program run under `nohup`, so it shut down when its
  terminal went away. v2 doesn't listen for a signal the program ignores.
- **Nothing is logged.** v1 logged "os.Interrupt - shutting down..." on the first signal and "os.Kill - terminating..."
  on the second, to the standard logger. Log in a stop, or after `GracefulStop` returns.
- **The second signal still exits with status 1**, at once, now without a message. When `ctx` started the shutdown,
  it takes two signals, so that one signal caught by `signal.NotifyContext` as well doesn't end the process.
- **Signal handling stops when GracefulStop returns.** v1 never undid its `signal.Notify`, which went unnoticed only
  because the process ended. After v2 returns, signals do what they did before it was called.
