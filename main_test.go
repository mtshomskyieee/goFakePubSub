
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestMainIntegration(t *testing.T) {
	// Start server in background
	go main()
	
	// Wait for server to start
	time.Sleep(2 * time.Second)
	
	// Prepare test message
	message := struct {
		Data string            `json:"data"`
		Attr map[string]string `json:"attributes,omitempty"`
	}{
		Data: "test message",
		Attr: map[string]string{"test": "true"},
	}
	
	// Marshal message to JSON
	body, err := json.Marshal(message)
	if err != nil {
		t.Fatalf("Failed to marshal message: %v", err)
	}
	
	// Send POST request to publish endpoint
	url := "http://0.0.0.0:8090/publish"
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}
	defer resp.Body.Close()
	
	// Check response status
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK, got %v", resp.Status)
	}
	
	fmt.Println("Test message sent successfully")
}
