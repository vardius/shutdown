/*
Package shutdown provides simple shutdown signals handler with callback

	package main

	import (
		"context"
		"net/http"
		"os"
		"time"
		"log"

		"github.com/vardius/shutdown"
	)

	func main() {
		ctx := context.Background()

		http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			io.WriteString(w, "Hello!\n")
		})

		stop := func() {
			ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			log.Printf("shutting down...\n")

			if err := srv.Shutdown(ctx); err != nil {
				log.Printf("shutdown error: %v\n", err)
			} else {
				log.Printf("gracefully stopped\n")
			}
		}

		go func() {
			log.Printf("%v\n", http.ListenAndServe(":8080", nil))
			stop()
			os.Exit(1)
		}()
	}
*/
package shutdown
