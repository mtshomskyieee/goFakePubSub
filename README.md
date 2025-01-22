# Mock PubSub Server

A Go-based mock PubSub server implementation designed for local development and testing environments. This server provides complete PubSub functionality including message publishing, subscription management, and real-time message delivery monitoring through a web interface.

## Features

- Full PubSub message publishing and receiving capabilities
- Real-time message monitoring through WebSocket connection
- Web interface for message visualization
- RESTful API for message publishing
- CORS support for cross-origin requests
- Automatic reconnection for WebSocket clients

## Prerequisites

- Go 1.21 or later 
- Web browser for accessing the monitoring interface
- Git (for local development only)

## Running the Server

### Local Development
1. Clone the repository
2. Navigate to the project directory
3. Install dependencies:
   ```bash
   go mod tidy
   ```
4. Run the server:
   ```bash
   go run main.go
   ```

## Port Configuration

By default, the server runs on port 8090. You can customize the port by setting the `PORT` environment variable:
```bash
PORT=8080 go run main.go
```

## Accessing the Frontend

### Local Development
When running locally, access the web interface at:
```
http://localhost:8090
```

The interface provides a real-time view of all published messages, including:
- Topic name
- Message ID
- Message data
- Message attributes
- Publish timestamp

## API Endpoints

### 1. Publish Message
```http
POST /publish
Content-Type: application/json

{
    "data": "Your message content",
    "attributes": {
        "key1": "value1",
        "key2": "value2"
    }
}
```

### 2. WebSocket Connection
```
ws://localhost:8090/ws
```
The WebSocket endpoint provides real-time updates of all published messages.

### 3. Health Check
```http
GET /health
```

## Example Usage

### Publishing a Message using curl

```bash
curl -X POST http://localhost:8090/publish \
  -H "Content-Type: application/json" \
  -d '{
    "data": "Hello, PubSub!",
    "attributes": {
      "priority": "high",
      "category": "greetings"
    }
  }'
```

## Development Notes

- The server creates a default topic "test-topic" and subscription "test-subscription" on startup
- Messages are stored in memory (last 100 messages)

## Troubleshooting

### Common Issues

1. **Port Already in Use**
   - Error: `port 8090 is in use`
   - Solution: Either stop the existing process using port 8090 or specify a different port using the PORT environment variable

2. **WebSocket Connection Failed**
   - Check if the server is running and accessible
   - Ensure you're using the correct WebSocket URL (ws:// for HTTP, wss:// for HTTPS)
   - Check browser console for detailed error messages

3. **Messages Not Appearing**
   - Verify the WebSocket connection is established (check browser console)
   - Ensure the publish request is sent with the correct JSON format
   - Check server logs for any message processing errors

### Getting Help

If you encounter any issues not covered here, please:
1. Check the server logs for error messages
2. Verify your request format matches the API documentation
3. Create an issue in the repository with detailed steps to reproduce the problem
- The web interface automatically reconnects if the connection is lost
- All endpoints support CORS for cross-origin requests

## Monitoring

1. Open the web interface in your browser
2. The interface will automatically connect to the WebSocket endpoint
3. Published messages will appear in real-time in the table
4. The connection status is handled automatically with reconnection logic

## Testing

Run the tests using:
```bash
go test ./...
```
