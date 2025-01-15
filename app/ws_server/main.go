package main

import (
	"github.com/gorilla/websocket"
	"log"
	"net/http"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	// Upgrade connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Error upgrading connection:", err)
		return
	}
	defer conn.Close()

	log.Println("New WebSocket connection established")

	for {
		// Read message from client
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			log.Println("Error reading message:", err)
			break
		}

		// Log message received from client
		log.Printf("Received message: %s\n", p)

		// Process message and send it back to client
		err = conn.WriteMessage(messageType, p)
		if err != nil {
			log.Println("Error writing message:", err)
			break
		}

		// Log message sent to client
		log.Printf("Sent message: %s\n", p)
	}
}

func main() {
	http.HandleFunc("/ws", handleConnections)
	log.Println("WebSocket server started on :8081")
	
	err := http.ListenAndServe(":8081", nil)
	if err != nil {
		log.Fatal("ListenAndServe error: ", err)
	}
}
