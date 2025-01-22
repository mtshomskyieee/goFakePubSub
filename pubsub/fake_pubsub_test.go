package pubsub

import (
    "context"
    "testing"
    "time"
)

func TestFakePubSub(t *testing.T) {
    ctx := context.Background()
    client := NewClient()
    defer client.Close()

    // Test topic creation
    topic, err := client.CreateTopic(ctx, "test-topic")
    if err != nil {
        t.Fatalf("Failed to create topic: %v", err)
    }

    // Test duplicate topic creation
    _, err = client.CreateTopic(ctx, "test-topic")
    if err == nil {
        t.Fatal("Expected error for duplicate topic creation")
    }

    // Test subscription creation
    sub, err := client.CreateSubscription(ctx, "test-sub", topic)
    if err != nil {
        t.Fatalf("Failed to create subscription: %v", err)
    }

    // Test message publishing and receiving
    testMessage := &Message{
        Data:       []byte("test message"),
        Attributes: map[string]string{"key": "value"},
    }

    messageReceived := make(chan bool)
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    // Start receiving messages
    go func() {
        err := sub.Receive(ctx, func(ctx context.Context, msg *Message) {
            if string(msg.Data) == "test message" {
                if msg.Attributes["key"] != "value" {
                    t.Errorf("Wrong attributes received")
                }
                msg.Ack()
                messageReceived <- true
            }
        })
        if err != nil && err != context.Canceled {
            t.Errorf("Receive error: %v", err)
        }
    }()

    // Publish message
    err = topic.Publish(ctx, testMessage)
    if err != nil {
        t.Fatalf("Failed to publish message: %v", err)
    }

    // Wait for message to be received
    select {
    case <-messageReceived:
        // Success
    case <-time.After(5 * time.Second):
        t.Fatal("Timeout waiting for message")
    }
}
