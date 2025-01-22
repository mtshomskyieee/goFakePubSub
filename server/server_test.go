package server

import (
    "bytes"
    "context"
    "encoding/json"
    "fakepubsub/pubsub"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"
)

func TestServer(t *testing.T) {
    client := pubsub.NewClient()
    defer client.Close()

    srv, err := NewServer(client, "test-topic", "test-sub")
    if err != nil {
        t.Fatalf("Failed to create server: %v", err)
    }

    // Start message processing in background
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    go srv.processMessages(ctx)

    // Create test message
    testMsg := Message{
        Data: "test message",
        Attr: map[string]string{"test": "attr"},
    }
    
    body, err := json.Marshal(testMsg)
    if err != nil {
        t.Fatalf("Failed to marshal message: %v", err)
    }

    // Create test request
    req := httptest.NewRequest(http.MethodPost, "/publish", bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    
    // Create response recorder
    rr := httptest.NewRecorder()

    // Handle request
    srv.handlePublish(rr, req)

    // Check response
    if rr.Code != http.StatusOK {
        t.Errorf("Handler returned wrong status code: got %v want %v",
            rr.Code, http.StatusOK)
    }

    // Allow time for message processing
    time.Sleep(100 * time.Millisecond)
}
