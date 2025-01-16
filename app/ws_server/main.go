package main

import (
	"context"
	"github.com/gorilla/websocket"
	"github.com/go-redis/redis/v8"
	"log"
	"net/http"
	"sync"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var clients = make(map[*websocket.Conn]*Client) // Save all websocket connections
var clientsMutex = sync.Mutex{}                 // Mutex to protect clients

var redisClient *redis.Client
var ctx = context.Background()

type Client struct {
	conn *websocket.Conn
	send chan []byte
}

// Connect to Redis
func initRedis() {
	redisClient = redis.NewClient(&redis.Options{
		Addr: "localhost:6379", // Redis server address
	})
	_, err := redisClient.Ping(ctx).Result()
	if err != nil {
		log.Fatal("Could not connect to Redis:", err)
	}
}

// Broadcast send message to all clients except the sender
func broadcast(senderConn *websocket.Conn, message []byte) {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()
	for _, client := range clients {
		// Skip sending message to the sender
		if client.conn == senderConn {
			continue
		}

		select {
		case client.send <- message:
		default:
			close(client.send)
			delete(clients, client.conn)
		}
	}
}

// Broadcast message to clients from Redis
func broadcastToClients(message []byte) {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()
	for _, client := range clients {
		select {
		case client.send <- message:
		default:
			close(client.send)
			delete(clients, client.conn)
		}
	}
}

func subscribeToRedis() {
	pubsub := redisClient.Subscribe(ctx, "chat")
	_, err := pubsub.Receive(ctx)
	if err != nil {
		log.Fatal("Error subscribing to Redis channel:", err)
	}

	ch := pubsub.Channel()
	for msg := range ch {
		log.Printf("Message received from Redis: %s", msg.Payload)

		// Broadcast to all clients (except the sender)
		broadcastToClients([]byte(msg.Payload))
	}
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	// Upgrade connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Error upgrading connection:", err)
		return
	}
	defer conn.Close()

	// Add connection to clients
	clients[conn] = &Client{
		conn: conn,
		send: make(chan []byte),
	}

	log.Println("New WebSocket connection established")

	// Listen for messages from the WebSocket
	go handleMessages(conn) // Handle incoming messages for this connection

	// Listen for messages from client
	for {
		// Read message from client
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			log.Println("Error reading message:", err)
			break
		}

		// Only publish to Redis if it's the first time
		if err := redisClient.Publish(ctx, "chat", string(p)).Err(); err != nil {
			log.Println("Error publishing message to Redis:", err)
			break
		}

		// Broadcast message to other clients
		broadcast(conn, p)

		// Resend message back to client
		err = conn.WriteMessage(messageType, p)
		if err != nil {
			log.Println("Error writing message:", err)
			break
		}
	}
}


// Handle the incoming message for a single connection
func handleMessages(conn *websocket.Conn) {
	for {
		// Receive message from the WebSocket connection
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			log.Println("Error reading message:", err)
			return
		}

		// Here we can also decide to handle message in a different way if needed
		log.Printf("Message received from WebSocket: %s", p)

		// Broadcast to other clients
		broadcast(conn, p)

		// Echo message back to the client
		err = conn.WriteMessage(messageType, p)
		if err != nil {
			log.Println("Error sending message to client:", err)
			return
		}
	}
}

func main() {
	// Initialize Redis client
	initRedis()

	// Start Redis subscriber in a separate goroutine
	go subscribeToRedis()

	// Start WebSocket server
	http.HandleFunc("/ws", handleConnections)
	log.Println("WebSocket server started on :8081")
	err := http.ListenAndServe(":8081", nil)
	if err != nil {
		log.Fatal("ListenAndServe error: ", err)
	}
}
