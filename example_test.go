package shutdown_test

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"syscall"
	"time"

	"github.com/vardius/shutdown"
)

func Example() {
	// mock shutdown signal Ctrl + C
	go func() {
		time.Sleep(10 * time.Millisecond)
		syscall.Kill(syscall.Getpid(), syscall.SIGINT)
	}()

	shutdown.GracefulStop(func() {
		fmt.Println("shutdown")
	})

	// Output:
	// shutdown
}

func Example_second() {
	// mock shutdown signal Ctrl + C followed by second Ctrl + C
	go func() {
		// first signal interrupt
		time.Sleep(10 * time.Millisecond)
		syscall.Kill(syscall.Getpid(), syscall.SIGINT)

		// second signal kill
		time.Sleep(10 * time.Millisecond)
		syscall.Kill(syscall.Getpid(), syscall.SIGINT)
	}()

	shutdown.GracefulStop(func() {
		time.Sleep(100 * time.Millisecond)
		fmt.Println("shutdown")
	})

	// Output:
}

func Example_third() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		fmt.Fprintf(w, "Hello!")
	})

	httpServer := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	stop := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(ctx); err != nil {
			log.Printf("shutdown error: %v\n", err)
		} else {
			log.Printf("gracefully stopped\n")
		}
	}

	// mock shutdown signal Ctrl + C
	go func() {
		// first signal interrupt
		time.Sleep(10 * time.Millisecond)
		syscall.Kill(syscall.Getpid(), syscall.SIGINT)
	}()

	shutdown.GracefulStop(stop)

	// Output:
	// gracefully stopped
}
