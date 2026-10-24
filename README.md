# ⏲️ shutdown

[![Test](https://github.com/vardius/shutdown/actions/workflows/test.yml/badge.svg)](https://github.com/vardius/shutdown/actions/workflows/test.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/vardius/shutdown/v2.svg)](https://pkg.go.dev/github.com/vardius/shutdown/v2)
[![Go Report Card](https://goreportcard.com/badge/github.com/vardius/shutdown/v2)](https://goreportcard.com/report/github.com/vardius/shutdown/v2)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE.md)

Stop a Go program gracefully. On SIGINT, SIGTERM or SIGHUP, run your stop functions with a deadline and get their errors back.

- **Returns.** `GracefulStop` returns the stops' errors instead of calling `os.Exit`, so `main` picks the exit status and its deferred calls run.
- **A deadline.** The stops share one context, done a set time after the shutdown starts.
- **In order.** Stops run one after another, in the order given. Every one is called, and every error is kept.
- **A way out.** A second signal ends the process at once, for when a stop hangs.
- **Go's defaults kept.** SIGQUIT still dumps every goroutine, and a signal the program ignores, such as SIGHUP under `nohup`, stays ignored.
- **Small.** One function and no dependencies beyond the standard library.

📖 **[Documentation](https://vardius.github.io/shutdown/)** · [API reference](https://pkg.go.dev/github.com/vardius/shutdown/v2) · [Examples](examples) · [Migrating from v1](MIGRATION.md)

## Install

```sh
go get github.com/vardius/shutdown/v2
```

Requires Go 1.26 or newer.

## Usage

```go
srv := &http.Server{Addr: ":8080", Handler: mux}
go func() {
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}()

// Blocks until a signal, then gives the requests in flight 10 seconds to finish.
if err := shutdown.GracefulStop(context.Background(), 10*time.Second, srv.Shutdown); err != nil {
	log.Fatal(err)
}
```

`GracefulStop` blocks until the process gets SIGINT (Ctrl+C), SIGTERM or SIGHUP. Then it calls `srv.Shutdown` with a context
that is done 10 seconds later, and returns its error, joined with `errors.Join`: match it with `errors.Is`.

## Stop functions

A stop is a `func(context.Context) error`, such as `srv.Shutdown`. Pass as many as you need:

```go
err := shutdown.GracefulStop(ctx, 30*time.Second,
	srv.Shutdown,
	queue.Drain,
	func(context.Context) error { return db.Close() },
)
```

They run one after another, in that order, and share one deadline: 30 seconds for all of them, from the moment the shutdown starts.
Every stop is called, even once the deadline has passed, so each should return early when its context is done.
`GracefulStop` returns their errors joined with `errors.Join`, in the same order, or nil.

The stops' context carries the values of the `ctx` you pass, but not its cancellation.

## Shutting down for your own reasons

`GracefulStop` also starts the shutdown when `ctx` is done. Cancel it when the program should stop without a signal,
for instance when the server fails: [examples/server](examples/server/main.go) does that, and returns the server's error.

## A second signal

The second signal `GracefulStop` receives ends the process at once with exit status 1. `GracefulStop` doesn't return,
and deferred calls don't run: it's the way out when a stop hangs. To see what it hangs on, press Ctrl+\ instead:
SIGQUIT is left to Go, which prints every goroutine's stack and exits.

`GracefulStop` counts signals, not shutdowns. When `ctx` started the shutdown, it takes two signals to end the process,
so the one signal that cancelled a context from `signal.NotifyContext`, and reached `GracefulStop` too, doesn't.

## Upgrading from v1

v1 logged, ran one function and called `os.Exit(0)`. v2 returns:

```go
// v1
shutdown.GracefulStop(func() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
})

// v2
if err := shutdown.GracefulStop(context.Background(), 10*time.Second, srv.Shutdown); err != nil {
	log.Printf("shutdown error: %v", err)
}
```

[MIGRATION.md](MIGRATION.md) maps the v1 call to its v2 form and lists the behaviour changes.

## Contributing

Bug reports and pull requests are welcome: see [CONTRIBUTING.md](.github/CONTRIBUTING.md).

## License

MIT, see [LICENSE.md](LICENSE.md).
