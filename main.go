package main

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"

    "fakepubsub/pubsub"
    "fakepubsub/server"
)

func main() {
    client := pubsub.NewClient()
    defer client.Close()

    log.Printf("Creating server with topic 'test-topic' and subscription 'test-subscription'")
    srv, err := server.NewServer(client, "test-topic", "test-subscription")
    if err != nil {
        log.Fatalf("Failed to create server: %v", err)
    }
    log.Printf("Server created successfully")

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Handle shutdown gracefully
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

    go func() {
        <-sigChan
        cancel()
    }()

    // Get port from environment or use default
    port := os.Getenv("PORT")
    if port == "" {
        port = "8091"
    }
    log.Printf("Starting server on port %s", port)
    
    errChan := make(chan error, 1)
    go func() {
        if err := srv.Start(ctx); err != nil && err != http.ErrServerClosed {
            errChan <- fmt.Errorf("server error: %v", err)
        }
    }()

    select {
    case <-ctx.Done():
        log.Println("Shutting down server...")
    case err := <-errChan:
        log.Printf("Error starting server: %v", err)
        os.Exit(1)
    }
}
