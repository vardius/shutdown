# Changelog

## v2.0.0 (2026-10-24)

`GracefulStop` returns now, and runs your stop functions with a deadline. See [MIGRATION.md](MIGRATION.md) for the before and after.

### Added

- A context: `GracefulStop` also starts the shutdown when it is done, for a reason of your own such as a failed server.
- A deadline: the stops share a context that is done `timeout` after the shutdown starts.
- Several stops: they run one after another, in the order given, and their errors come back joined.
- A runnable example in `examples/server`, with a test: an HTTP server that drains on SIGTERM.

### Changed

- Module path is `github.com/vardius/shutdown/v2`; Go 1.26 or newer is required.
- `GracefulStop(ctx, timeout, stops...) error` returns the stops' errors instead of calling `os.Exit(0)`,
  so code after it and deferred calls run.
- SIGQUIT is no longer caught: Go prints every goroutine's stack and exits, as it does by default.
- A signal the program ignores stays ignored, so a program run under `nohup` survives its terminal going away.
- When the context started the shutdown, it takes two signals to end the process.
- Signal handling stops when `GracefulStop` returns.
- A nil stop panics when `GracefulStop` is called, not when the shutdown reaches it.

### Removed

- The log lines on the first and the second signal.

### Fixed

- The package documentation's example compiles.
