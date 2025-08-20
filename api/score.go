package handler

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Upgrader configures the WebSocket connection
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all connections for this example
		return true
	},
}

// Keep track of all connected clients
var clients = make(map[*websocket.Conn]bool)
var mu sync.Mutex // To protect concurrent access to clients map

// Handler is the entry point for the Vercel Serverless Function
func Handler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	defer conn.Close()

	// Register new client
	mu.Lock()
	clients[conn] = true
	mu.Unlock()

	log.Println("Client connected")

	// The broadcast function needs to be triggered elsewhere in a real app.
	// For this example, we start a goroutine to simulate score updates.
	// NOTE: In a true serverless environment, this goroutine might not persist.
	// A better approach would be to use a database or a pub/sub system to trigger updates.
	go broadcastScoreUpdates()

	// Keep the connection alive by reading messages (but do nothing with them)
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			log.Println("Read error:", err)
			mu.Lock()
			delete(clients, conn)
			mu.Unlock()
			break
		}
	}
}

// broadcastScoreUpdates simulates a live score update every 5 seconds
func broadcastScoreUpdates() {
	// Simple score simulator
	homeScore := 0
	awayScore := 0
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// Randomly update a score
		if time.Now().Unix()%2 == 0 {
			homeScore++
		} else {
			awayScore++
		}

		score := fmt.Sprintf(`{"home": %d, "away": %d}`, homeScore, awayScore)

		mu.Lock()
		// Send the new score to all connected clients
		for client := range clients {
			err := client.WriteMessage(websocket.TextMessage, []byte(score))
			if err != nil {
				log.Printf("Write error: %v", err)
				client.Close()
				delete(clients, client)
			}
		}
		mu.Unlock()
	}
}
