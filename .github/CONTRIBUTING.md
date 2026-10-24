# Contributing

Thanks for helping improve **shutdown**. By contributing, you agree to abide by the [code of conduct](CODE_OF_CONDUCT.md).

## Issues and questions

Search the [issue tracker](https://github.com/vardius/shutdown/issues) first; if nothing matches, open an issue.
Bugs, ideas and usage questions are all welcome. For a bug, include the stops you passed, the signals the process got,
and what you expected to happen.

## Development

You need Go 1.26 or newer.

```sh
git clone https://github.com/vardius/shutdown.git
cd shutdown
go test ./...
```

Before sending a pull request, run what CI runs:

```sh
gofmt -l .
go vet ./...
GOOS=windows go vet ./...
go run honnef.co/go/tools/cmd/staticcheck@v0.8.2 ./...
go test -race -shuffle=on ./...
```

### Layout

- `shutdown.go` holds `GracefulStop`; `doc.go` is the package documentation.
- `shutdown_test.go` runs on a fake clock with `testing/synctest`, with fake signals and a fake exit, so timings are exact and
  nothing sleeps for real. Keep new tests that way.
- `signal_unix_test.go` sends real signals to the test binary, each only once `GracefulStop` has registered for it, and runs
  one test in a child process, for SIGQUIT. No test calls the real `os.Exit`.
- `examples/` holds runnable programs, each with a test. The website shows them, so keep them short.
- `website/` is the documentation site.

### Website

```sh
cd website
npm ci
npm run dev
```

## Pull requests

- For a large change, open an issue first so we can agree on the approach.
- Branch from `master`, keep the change focused, and add a test that fails without it.
