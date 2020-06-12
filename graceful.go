/*
Package shutdown provides simple shutdown signals handler with callback handler
*/
package shutdown

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

// GracefulStop handles signal and graceful shutdown by executing callback function
// when signal is received callback is called followed after by os.Exit(0), it is responsibility of callback to handle timeout
// if second signal is received will terminate process by a call to os.Exit(1)
func GracefulStop(stop func()) {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(
		signalChan,
		syscall.SIGHUP,  // kill -SIGHUP XXXX
		syscall.SIGINT,  // kill -SIGINT XXXX or Ctrl+c
		syscall.SIGTERM, // kill -SIGTERM XXXX
		syscall.SIGQUIT, // kill -SIGQUIT XXXX
	)

	<-signalChan
	log.Print("os.Interrupt - shutting down...\n")

	// terminate after second signal before callback is done
	go func() {
		<-signalChan
		log.Fatal("os.Kill - terminating...\n")
	}()

	stop()

	os.Exit(0)
}
