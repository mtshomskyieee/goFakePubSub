package pubsub

import (
    "time"
)

// Message represents a PubSub message
type Message struct {
    ID          string
    Data        []byte
    Attributes  map[string]string
    PublishTime time.Time
    acked       bool
}

// Ack acknowledges the message
func (m *Message) Ack() {
    m.acked = true
}

// Nack negative-acknowledges the message
func (m *Message) Nack() {
    m.acked = false
}
