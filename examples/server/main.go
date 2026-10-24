// Command server answers HTTP requests until it gets SIGINT, SIGTERM or SIGHUP. Then it stops taking connections,
// gives the requests in flight 10 seconds to finish, and cuts off any still running.
//
//	go run ./examples/server
package main

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/vardius/shutdown/v2"
)

// serve answers requests on ln until the process gets a shutdown signal, ctx is done, or the server fails.
func serve(ctx context.Context, ln net.Listener, h http.Handler, grace time.Duration) error {
	srv := &http.Server{Handler: h, ReadHeaderTimeout: 10 * time.Second}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	served := make(chan error, 1)
	go func() {
		served <- srv.Serve(ln)
		cancel() // if the server failed, shut down too
	}()

	err := shutdown.GracefulStop(ctx, grace,
		srv.Shutdown, // close the listener, and wait for the requests in flight
		func(context.Context) error { return srv.Close() }, // cut off any still running
	)
	if serveErr := <-served; !errors.Is(serveErr, http.ErrServerClosed) {
		return errors.Join(serveErr, err)
	}
	return err
}

// work takes three seconds to answer, so that there is something to wait for.
func work(w http.ResponseWriter, r *http.Request) {
	select {
	case <-time.After(3 * time.Second):
		io.WriteString(w, "done\n")
	case <-r.Context().Done(): // cut off, or the client went away
	}
}

func main() {
	log.SetFlags(0)
	ln, err := net.Listen("tcp", "localhost:8080")
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("listening on http://%s", ln.Addr())

	if err := serve(context.Background(), ln, http.HandlerFunc(work), 10*time.Second); err != nil {
		log.Fatal(err)
	}
	log.Print("stopped")
}
