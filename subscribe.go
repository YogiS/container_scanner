package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/events"
	"github.com/containerd/containerd/namespaces"
)

func main() {
	// Create a new context
	ctx := namespaces.WithNamespace(context.Background(), "default")

	// Connect to the local containerd daemon
	client, err := containerd.New("/run/containerd/containerd.sock")
	if err != nil {
		log.Fatalf("Failed to connect to containerd: %v", err)
	}
	defer client.Close()

	// Subscribe to containerd events
	eventCh, errCh := client.Subscribe(ctx)

	// Create a signal channel to gracefully exit on interrupt
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)

	// Handle events and signals in a loop
	for {
		select {
		case event := <-eventCh:
			// Handle the received event
			handleEvent(event)

		case err := <-errCh:
			// Handle errors from the event subscription
			log.Printf("Error in event subscription: %v", err)

		case <-signalCh:
			// Handle interrupt signal for graceful exit
			fmt.Println("Received interrupt signal. Exiting...")
			return
		}
	}
}

// handleEvent prints information about the received event
func handleEvent(event *events.Envelope) {
	fmt.Printf("Received event:\n")
	fmt.Printf("  Topic: %s\n", event.Topic)
	fmt.Printf("  Namespace: %s\n", event.Namespace)
	fmt.Printf("\n")
}
