package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// Upgrader converts HTTP connections to WebSocket connections.
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Client represents a single connected user.
type Client struct {
	conn *websocket.Conn
}

// GameState holds the current score. The mutex ensures safe concurrent access.
type GameState struct {
	mu     sync.Mutex
	ScoreA int `json:"scoreA"`
	ScoreB int `json:"scoreB"`
}

// Message represents an incoming command from a client.
type Message struct {
	Action string `json:"action"` // e.g., "increment", "decrement", "reset"
	Team   string `json:"team"`   // e.g., "A", "B"
}

// Hub maintains the set of active clients and broadcasts messages.
type Hub struct {
	clients map[*Client]bool
	mutex   sync.Mutex
}

var hub = Hub{clients: make(map[*Client]bool)}
var gameState = GameState{ScoreA: 0, ScoreB: 0}

// broadcast sends a message to all connected clients.
func (h *Hub) broadcast(message []byte) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	for client := range h.clients {
		err := client.conn.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			log.Printf("broadcast error: %v", err)
			client.conn.Close()
			delete(h.clients, client)
		}
	}
}

// handleMessages processes incoming messages from a client.
func handleMessages(client *Client) {
	defer func() {
		hub.mutex.Lock()
		delete(hub.clients, client)
		hub.mutex.Unlock()
		client.conn.Close()
		log.Println("Client disconnected")
	}()

	for {
		_, payload, err := client.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("read error: %v", err)
			}
			break
		}

		var msg Message
		if err := json.Unmarshal(payload, &msg); err != nil {
			log.Printf("json unmarshal error: %v", err)
			continue
		}

		// Lock the game state while we modify it
		gameState.mu.Lock()
		switch msg.Action {
		case "increment":
			if msg.Team == "A" {
				gameState.ScoreA++
			} else if msg.Team == "B" {
				gameState.ScoreB++
			}
		case "decrement":
			if msg.Team == "A" && gameState.ScoreA > 0 {
				gameState.ScoreA--
			} else if msg.Team == "B" && gameState.ScoreB > 0 {
				gameState.ScoreB--
			}
		case "reset":
			gameState.ScoreA = 0
			gameState.ScoreB = 0
		}

		// Marshal the updated state to JSON
		updatedState, _ := json.Marshal(gameState)
		gameState.mu.Unlock()

		// Broadcast the new state to everyone
		hub.broadcast(updatedState)
		log.Printf("Processed message: %+v. New state: %s", msg, updatedState)
	}
}

func serveWs(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	client := &Client{conn: conn}
	hub.mutex.Lock()
	hub.clients[client] = true
	hub.mutex.Unlock()

	log.Println("New client connected")

	// Send the initial state to the newly connected client
	gameState.mu.Lock()
	initialState, _ := json.Marshal(gameState)
	gameState.mu.Unlock()
	client.conn.WriteMessage(websocket.TextMessage, initialState)

	// Listen for messages from this client in a new goroutine
	go handleMessages(client)
}

func main() {
	http.HandleFunc("/ws", serveWs)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	log.Println("Server starting on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
