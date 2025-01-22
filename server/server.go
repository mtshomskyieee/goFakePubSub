package server

import (
    "context"
    "encoding/json"
    "fmt"
    "html/template"
    "log"
    "net"
    "net/http"
    "os"
    "sync"
    "time"

    "fakepubsub/pubsub"
    "github.com/gorilla/websocket"
)

// Server represents the HTTP server that handles PubSub messages
type Server struct {
    client    *pubsub.Client
    topic     *pubsub.Topic
    sub       *pubsub.Subscription
    mu        sync.RWMutex
    messages  []*pubsub.Message
    clients   map[*websocket.Conn]bool
    upgrader  websocket.Upgrader
}

// Message represents the incoming message format
type Message struct {
    Data string            `json:"data"`
    Attr map[string]string `json:"attributes,omitempty"`
}

// NewServer creates a new server instance
func NewServer(client *pubsub.Client, topicID, subID string) (*Server, error) {
    ctx := context.Background()

    log.Printf("Creating topic: %s", topicID)
    topic, err := client.CreateTopic(ctx, topicID)
    if err != nil {
        return nil, fmt.Errorf("failed to create topic: %v", err)
    }
    log.Printf("Topic created successfully: %s", topicID)

    log.Printf("Creating subscription: %s for topic: %s", subID, topicID)
    sub, err := client.CreateSubscription(ctx, subID, topic)
    if err != nil {
        return nil, fmt.Errorf("failed to create subscription: %v", err)
    }
    log.Printf("Subscription created successfully: %s", subID)

    srv := &Server{
        client:   client,
        topic:    topic,
        sub:      sub,
        messages: make([]*pubsub.Message, 0),
        clients:  make(map[*websocket.Conn]bool),
        upgrader: websocket.Upgrader{
            CheckOrigin: func(r *http.Request) bool {
                return true // Allow all origins in development
            },
            ReadBufferSize:  1024,
            WriteBufferSize: 1024,
            HandshakeTimeout: 10 * time.Second,
            EnableCompression: true,
        },
    }
    
    log.Printf("Server initialized with WebSocket support")
    return srv, nil
}

// Start starts the HTTP server and message processing
func (s *Server) Start(ctx context.Context) error {
    // Get current working directory for logging
    pwd, err := os.Getwd()
    if err != nil {
        log.Printf("Warning: Could not get working directory: %v", err)
    } else {
        log.Printf("Current working directory: %s", pwd)
    }

    log.Printf("Starting message processing routine...")
    go s.processMessages(ctx)

    log.Printf("Initializing HTTP server...")
    // Get port from environment or use default
    port := os.Getenv("PORT")
    if port == "" {
        port = "8090"
    }
    addr := fmt.Sprintf("0.0.0.0:%s", port)

    // Log template information
    if _, err := os.Stat("templates/index.html"); err != nil {
        log.Printf("Warning: Template file not found at templates/index.html: %v", err)
        // Try alternate path
        if _, err := os.Stat("../templates/index.html"); err != nil {
            log.Printf("Warning: Template also not found at ../templates/index.html: %v", err)
        }
    } else {
        log.Printf("Template file found at templates/index.html")
    }

    // Log startup configuration
    log.Printf("Server Configuration:")
    log.Printf("- Listening Address: %s", addr)
    log.Printf("- WebSocket Endpoint: ws://%s/ws", addr)
    log.Printf("- Topic Name: %s", s.topic.Name())

    // Create a new http.Server instance with more detailed configuration
    mux := http.NewServeMux()

    // Add CORS middleware for all routes
    withCORS := func(h http.HandlerFunc) http.HandlerFunc {
        return func(w http.ResponseWriter, r *http.Request) {
            w.Header().Set("Access-Control-Allow-Origin", "*")
            w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
            w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
            
            if r.Method == "OPTIONS" {
                w.WriteHeader(http.StatusOK)
                return
            }
            
            h(w, r)
        }
    }

    // Apply CORS middleware to all routes
    mux.HandleFunc("/", withCORS(s.handleIndex))
    mux.HandleFunc("/ws", withCORS(s.handleWS))
    mux.HandleFunc("/publish", withCORS(s.handlePublish))
    mux.HandleFunc("/health", withCORS(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
    }))

    httpServer := &http.Server{
        Addr:    addr,
        Handler: mux,
        ReadTimeout:  15 * time.Second,
        WriteTimeout: 15 * time.Second,
    }

    // Channel to capture server start errors
    errChan := make(chan error, 1)

    // Start the server in a goroutine with retry logic
    go func() {
        maxRetries := 3
        retryDelay := time.Second * 2

        for i := 0; i < maxRetries; i++ {
            log.Printf("Attempt %d/%d: Starting HTTP server on %s", i+1, maxRetries, addr)
            
            ln, err := net.Listen("tcp", addr)
            if err != nil {
                if i < maxRetries-1 {
                    log.Printf("Port %s is busy, retrying in %v...", port, retryDelay)
                    time.Sleep(retryDelay)
                    continue
                }
                errChan <- fmt.Errorf("failed to bind to port %s after %d attempts: %v", port, maxRetries, err)
                return
            }

            log.Printf("Successfully bound to port %s", port)
            
            if err := httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
                if opErr, ok := err.(*net.OpError); ok && opErr.Op == "listen" {
                    if i < maxRetries-1 {
                        log.Printf("Port conflict detected, retrying...")
                        time.Sleep(retryDelay)
                        continue
                    }
                    errChan <- fmt.Errorf("port %s is in use, please set a different PORT environment variable", port)
                } else {
                    errChan <- err
                }
                return
            }
            return // Server started successfully
        }
    }()

    // Give the server a moment to start
    time.Sleep(100 * time.Millisecond)
    log.Printf("HTTP server is ready and listening on %s", addr)

    // Wait for context cancellation or server error
    select {
    case <-ctx.Done():
        log.Printf("Context cancelled, shutting down server...")
        shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        return httpServer.Shutdown(shutdownCtx)
    case err := <-errChan:
        return fmt.Errorf("server error: %v", err)
    }
}

func (s *Server) handlePublish(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    var msg Message
    if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    pubsubMsg := &pubsub.Message{
        ID:         fmt.Sprintf("msg-%d", time.Now().UnixNano()),
        Data:       []byte(msg.Data),
        Attributes: msg.Attr,
        PublishTime: time.Now(),
    }

    ctx := context.Background()
    if err := s.topic.Publish(ctx, pubsubMsg); err != nil {
        http.Error(w, fmt.Sprintf("Failed to publish message: %v", err), http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{"status": "published"})
}

func (s *Server) processMessages(ctx context.Context) {
    err := s.sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
        log.Printf("Received message: %s", string(msg.Data))
        
        s.mu.Lock()
        // Keep only the last 100 messages
        if len(s.messages) >= 100 {
            s.messages = s.messages[1:]
        }
        s.messages = append(s.messages, msg)
        s.mu.Unlock()
        
        s.broadcastMessages()
        msg.Ack()
    })

    if err != nil && err != context.Canceled {
        log.Printf("Error processing messages: %v", err)
    }
}

type wsMessage struct {
    Topic       string            `json:"topic"`
    ID          string            `json:"id"`
    Data        string            `json:"data"`
    Attributes  map[string]string `json:"attributes"`
    PublishTime time.Time         `json:"publish_time"`
}

func (s *Server) broadcastMessages() {
    s.mu.RLock()
    messages := make([]wsMessage, 0, len(s.messages))
    for _, msg := range s.messages {
        messages = append(messages, wsMessage{
            Topic:       s.topic.Name(),
            ID:          msg.ID,
            Data:        string(msg.Data),
            Attributes:  msg.Attributes,
            PublishTime: msg.PublishTime,
        })
    }
    s.mu.RUnlock()

    messageJSON, err := json.Marshal(messages)
    if err != nil {
        log.Printf("Error marshaling messages: %v", err)
        return
    }

    s.mu.RLock()
    for client := range s.clients {
        if err := client.WriteMessage(websocket.TextMessage, messageJSON); err != nil {
            log.Printf("Error sending message to client: %v", err)
            client.Close()
            delete(s.clients, client)
        }
    }
    s.mu.RUnlock()
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
    log.Printf("Received WebSocket connection request from %s", r.RemoteAddr)
    
    // Set CORS headers for WebSocket handshake
    w.Header().Set("Access-Control-Allow-Origin", "*")
    w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
    w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
    
    if r.Method == "OPTIONS" {
        w.WriteHeader(http.StatusOK)
        return
    }
    
    log.Printf("Attempting WebSocket upgrade for %s", r.RemoteAddr)
    conn, err := s.upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Printf("Error upgrading WebSocket connection: %v", err)
        http.Error(w, "Could not open websocket connection", http.StatusBadRequest)
        return
    }
    
    log.Printf("WebSocket connection successfully established with %s", r.RemoteAddr)
    
    s.mu.Lock()
    s.clients[conn] = true
    clientCount := len(s.clients)
    s.mu.Unlock()
    
    log.Printf("Current WebSocket clients: %d", clientCount)
    
    // Send initial messages to the new client
    s.broadcastMessages()
    
    // Setup ping/pong handlers
    conn.SetPingHandler(func(appData string) error {
        return conn.WriteControl(websocket.PongMessage, []byte{}, time.Now().Add(time.Second))
    })
    
    // Setup connection close handler
    conn.SetCloseHandler(func(code int, text string) error {
        log.Printf("WebSocket connection closing: %s, code: %d, message: %s", r.RemoteAddr, code, text)
        s.mu.Lock()
        delete(s.clients, conn)
        s.mu.Unlock()
        return nil
    })
    
    // Start a goroutine to handle connection monitoring
    go func() {
        for {
            _, _, err := conn.ReadMessage()
            if err != nil {
                log.Printf("WebSocket read error for %s: %v", r.RemoteAddr, err)
                s.mu.Lock()
                delete(s.clients, conn)
                s.mu.Unlock()
                return
            }
        }
    }()
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
    log.Printf("Attempting to load template file...")
    
    // Try different possible template paths
    templatePaths := []string{
        "templates/index.html",
        "../templates/index.html",
        "./templates/index.html",
    }
    
    var tmpl *template.Template
    var err error
    var loadedPath string
    
    for _, path := range templatePaths {
        log.Printf("Trying template path: %s", path)
        if _, statErr := os.Stat(path); statErr == nil {
            tmpl, err = template.ParseFiles(path)
            if err == nil {
                loadedPath = path
                break
            }
        }
    }
    
    if tmpl == nil {
        log.Printf("Error: Could not find template file in any of the expected locations")
        http.Error(w, "Template file not found", http.StatusInternalServerError)
        return
    }
    
    log.Printf("Template loaded successfully from: %s", loadedPath)
    
    err = tmpl.Execute(w, nil)
    if err != nil {
        log.Printf("Error executing template: %v", err)
        http.Error(w, fmt.Sprintf("Error executing template: %v", err), http.StatusInternalServerError)
        return
    }
    
    log.Printf("Template rendered successfully")
}
