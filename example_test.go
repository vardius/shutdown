package shutdown_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/vardius/shutdown/v2"
)

func ExampleGracefulStop() {
	// A program passes context.Background(), so that only a signal starts the shutdown.
	// Here a cancelled context stands in for the signal.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := shutdown.GracefulStop(ctx, 5*time.Second,
		func(ctx context.Context) error {
			fmt.Println("stop taking requests")
			return nil
		},
		func(ctx context.Context) error {
			fmt.Println("close the database")
			return errors.New("database: connection reset")
		},
	)

	fmt.Println(err)
	// Output:
	// stop taking requests
	// close the database
	// database: connection reset
}

func ExampleGracefulStop_timeout() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := shutdown.GracefulStop(ctx, 50*time.Millisecond, func(ctx context.Context) error {
		<-ctx.Done() // a stop that would take longer than the timeout
		return fmt.Errorf("flush queue: %w", ctx.Err())
	})

	fmt.Println(err)
	fmt.Println(errors.Is(err, context.DeadlineExceeded))
	// Output:
	// flush queue: context deadline exceeded
	// true
}

// On SIGINT, SIGTERM or SIGHUP, the server stops taking connections and gives the requests in flight
// 10 seconds to finish. A second signal ends the process at once.
func ExampleGracefulStop_httpServer() {
	srv := &http.Server{Addr: ":8080", Handler: http.DefaultServeMux}
	go func() {
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	if err := shutdown.GracefulStop(context.Background(), 10*time.Second, srv.Shutdown); err != nil {
		log.Fatal(err)
	}
}
