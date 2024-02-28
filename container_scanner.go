package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/events"
	"github.com/containerd/containerd/namespaces"
)

func main() {
	// Create a containerd client
	client, err := containerd.New("/run/containerd/containerd.sock")
	if err != nil {
		log.Fatalf("Failed to connect to containerd: %v", err)
	}
	defer client.Close()

	// Create a context with the "k8s.io" namespace
	ctx := namespaces.WithNamespace(context.Background(), "k8s.io")

	// Create a wait group to wait for goroutines to finish
	var wg sync.WaitGroup

	// Create a channel to handle termination signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Subscribe to container events
	eventCh, err := client.Subscribe(ctx, events.WithFilter("topic==container"))
	if err != nil {
		log.Fatalf("Failed to subscribe to container events: %v", err)
	}

	// Handle container events in a separate goroutine
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case e := <-eventCh:
				if e.Namespace != "k8s.io" {
					// Skip events from other namespaces
					continue
				}

				// Retrieve container information
				container, err := client.LoadContainer(ctx, e.ID)
				if err != nil {
					log.Printf("Error loading container %s: %v", e.ID, err)
					continue
				}

				// Skip non-running containers
				task, err := container.Task(ctx, cio.Load)
				if err != nil {
					log.Printf("Error getting task for container %s: %v", e.ID, err)
					continue
				}

				if task.Status(ctx).Status != containerd.Running {
					continue
				}

				// Handle container creation event for running containers
				wg.Add(1)
				go func(containerID string) {
					defer wg.Done()

					// Retrieve container information
					container, err := client.LoadContainer(ctx, containerID)
					if err != nil {
						log.Printf("Error loading container %s: %v", containerID, err)
						return
					}

					name, _ := container.Labels(ctx)

					// Print container information
					fmt.Printf("Running container created: Name=%s, ID=%s, PID=%d\n", name, containerID, task.Pid())
				}(e.ID)
			}
		}
	}()

	fmt.Println("Waiting for running container creation events...")

	// Wait for termination signals
	select {
	case <-sigCh:
		fmt.Println("Received termination signal. Cleaning up...")
		cancel := make(chan struct{})
		close(cancel)
		client.Unsubscribe(eventCh)
		close(eventCh)
		client.EventService().Close()
		wg.Wait()
		fmt.Println("Cleanup completed. Exiting.")
	}
}
