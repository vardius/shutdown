package shutdown_test

import (
	"fmt"
	"syscall"
	"time"

	"github.com/vardius/shutdown"
)

func Example() {
	// mock shutdown signall Ctrl + C
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
	// mock shutdown signall Ctrl + C followed by second Ctrl + C
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
