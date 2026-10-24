// Package shutdown stops a Go program gracefully: on SIGINT (Ctrl+C), SIGTERM or SIGHUP, it runs your stop
// functions with a deadline and returns their errors, so main decides how to exit and its deferred calls run.
//
//	mux := http.NewServeMux()
//	srv := &http.Server{Addr: ":8080", Handler: mux}
//	go func() {
//		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
//			log.Fatal(err)
//		}
//	}()
//
//	// Blocks until a signal, then gives the requests in flight 10 seconds to finish.
//	if err := shutdown.GracefulStop(context.Background(), 10*time.Second, srv.Shutdown); err != nil {
//		log.Fatal(err)
//	}
//
// # Starting the shutdown
//
// GracefulStop blocks until the process gets SIGINT, SIGTERM or SIGHUP, or the context passed to it is done:
// cancel that context to shut down for a reason of your own, such as a server that failed.
//
// SIGQUIT is left alone, so Ctrl+\ still makes Go print every goroutine's stack and exit: the way to see what a
// hung shutdown is waiting for. Signals the program ignores stay ignored, so a program run under nohup keeps
// running when its terminal goes away.
//
// # Stop functions
//
// The stops run one after another, in the order given, on the goroutine that called GracefulStop. They share one
// context, which is done timeout after the shutdown starts and carries the values of the context passed in, but
// not its cancellation. Every stop is called, even after the deadline, so each should return early once its
// context is done. GracefulStop returns their errors joined with errors.Join, in the same order, or nil.
//
// # A second signal
//
// The second signal GracefulStop receives ends the process at once, with exit status 1: GracefulStop doesn't
// return and deferred calls don't run. It is the way out when a stop hangs.
//
// GracefulStop counts signals, not shutdowns. When the context starts the shutdown, it takes two signals to end
// the process; that way the one signal that cancelled a context made with signal.NotifyContext, and reached
// GracefulStop as well, doesn't end it.
//
// GracefulStop stops listening for signals before it returns.
package shutdown
