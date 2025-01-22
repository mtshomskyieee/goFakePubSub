package pubsub

import (
    "context"
    "fmt"
    "sync"
    "time"
)

// Client represents a fake PubSub client
type Client struct {
    topics    map[string]*Topic
    mu        sync.RWMutex
    closed    bool
    closeOnce sync.Once
}

// Topic represents a PubSub topic
type Topic struct {
    name        string
    client      *Client
    subscribers map[string]*Subscription
    mu          sync.RWMutex
}

// Name returns the topic name
func (t *Topic) Name() string {
    return t.name
}

// Subscription represents a PubSub subscription
type Subscription struct {
    name     string
    topic    *Topic
    messages chan *Message
    mu       sync.RWMutex
}

// NewClient creates a new fake PubSub client
func NewClient() *Client {
    return &Client{
        topics: make(map[string]*Topic),
    }
}

// CreateTopic creates a new topic
func (c *Client) CreateTopic(ctx context.Context, topicID string) (*Topic, error) {
    c.mu.Lock()
    defer c.mu.Unlock()

    if c.closed {
        return nil, fmt.Errorf("client is closed")
    }

    if _, exists := c.topics[topicID]; exists {
        return nil, fmt.Errorf("topic already exists: %s", topicID)
    }

    topic := &Topic{
        name:        topicID,
        client:      c,
        subscribers: make(map[string]*Subscription),
    }
    c.topics[topicID] = topic
    return topic, nil
}

// Topic returns an existing topic
func (c *Client) Topic(id string) *Topic {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return c.topics[id]
}

// CreateSubscription creates a new subscription for a topic
func (c *Client) CreateSubscription(ctx context.Context, subID string, topic *Topic) (*Subscription, error) {
    if topic == nil {
        return nil, fmt.Errorf("topic cannot be nil")
    }

    topic.mu.Lock()
    defer topic.mu.Unlock()

    if _, exists := topic.subscribers[subID]; exists {
        return nil, fmt.Errorf("subscription already exists: %s", subID)
    }

    sub := &Subscription{
        name:     subID,
        topic:    topic,
        messages: make(chan *Message, 100), // Buffer size of 100 messages
    }
    topic.subscribers[subID] = sub
    return sub, nil
}

// Publish publishes a message to a topic
func (t *Topic) Publish(ctx context.Context, msg *Message) error {
    t.mu.RLock()
    defer t.mu.RUnlock()

    if msg.PublishTime.IsZero() {
        msg.PublishTime = time.Now()
    }

    for _, sub := range t.subscribers {
        select {
        case sub.messages <- msg:
        default:
            return fmt.Errorf("message buffer full for subscription: %s", sub.name)
        }
    }
    return nil
}

// Receive starts receiving messages from the subscription
func (s *Subscription) Receive(ctx context.Context, f func(context.Context, *Message)) error {
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case msg := <-s.messages:
            f(ctx, msg)
        }
    }
}

// Close closes the client and all associated resources
func (c *Client) Close() {
    c.closeOnce.Do(func() {
        c.mu.Lock()
        defer c.mu.Unlock()
        c.closed = true
        
        for _, topic := range c.topics {
            topic.mu.Lock()
            for _, sub := range topic.subscribers {
                close(sub.messages)
            }
            topic.mu.Unlock()
        }
    })
}
