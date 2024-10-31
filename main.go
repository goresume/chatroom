package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// Client represents a connected user
type Client struct {
	conn     *websocket.Conn
	send     chan []byte
	username string
}

// ChatRoom manages the chatroom state and client connections
type ChatRoom struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mutex      sync.RWMutex
}

// Message represents the structure of chat messages
type Message struct {
	Username string `json:"username"`
	Content  string `json:"content"`
	Type     string `json:"type,omitempty"` // "system" for system messages, empty for user messages
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // In production, implement proper origin checking
	},
}

func newChatRoom() *ChatRoom {
	return &ChatRoom{
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}
}

func (cr *ChatRoom) sendSystemMessage(content string) {
	message := Message{
		Username: "System",
		Content:  content,
		Type:     "system",
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		log.Printf("Error marshaling system message: %v", err)
		return
	}

	cr.broadcast <- messageBytes
}

func (cr *ChatRoom) run() {
	for {
		select {
		case client := <-cr.register:
			cr.mutex.Lock()
			cr.clients[client] = true
			cr.mutex.Unlock()

			go cr.sendSystemMessage(fmt.Sprintf("%s has joined the chat", client.username))
			log.Printf("New client connected. Total clients: %d", len(cr.clients))

		case client := <-cr.unregister:
			cr.mutex.Lock()
			if _, ok := cr.clients[client]; ok {
				delete(cr.clients, client)
				close(client.send)

				go cr.sendSystemMessage(fmt.Sprintf("%s has left the chat", client.username))
			}
			cr.mutex.Unlock()
			log.Printf("Client disconnected. Total clients: %d", len(cr.clients))

		case message := <-cr.broadcast:
			cr.mutex.RLock()
			for client := range cr.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(cr.clients, client)
				}
			}
			cr.mutex.RUnlock()
		}
	}
}

func (c *Client) readPump(cr *ChatRoom) {
	defer func() {
		cr.unregister <- c
		c.conn.Close()
	}()

	for {
		_, messageBytes, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}

		// Parse the incoming message to add the username
		var message Message
		if err := json.Unmarshal(messageBytes, &message); err != nil {
			log.Printf("Error unmarshaling message: %v", err)
			continue
		}

		// Ensure the username in the message matches the client's username
		message.Username = c.username

		// Marshal the message back to JSON
		messageBytes, err = json.Marshal(message)
		if err != nil {
			log.Printf("Error marshaling message: %v", err)
			continue
		}

		cr.broadcast <- messageBytes
	}
}

func (c *Client) writePump() {
	defer c.conn.Close()

	for {
		message, ok := <-c.send
		if !ok {
			c.conn.WriteMessage(websocket.CloseMessage, []byte{})
			return
		}

		err := c.conn.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			return
		}
	}
}

func serveWs(cr *ChatRoom, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	username := r.URL.Query().Get("username")
	if username == "" {
		username = "Anonymous"
	}

	client := &Client{
		conn:     conn,
		send:     make(chan []byte, 256),
		username: username,
	}
	cr.register <- client

	go client.writePump()
	go client.readPump(cr)
}

func main() {
	chatRoom := newChatRoom()
	go chatRoom.run()

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(chatRoom, w, r)
	})

	port := ":8080"
	fmt.Printf("Server starting on port %s\n", port)
	err := http.ListenAndServe(port, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
