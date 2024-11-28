package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"realtime-chat/database"
)

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

var (
	clients    = make(map[string]*websocket.Conn)
	users      = make(map[string]User)
	clientsMux sync.Mutex
)

type User struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname"`
}

type Message struct {
	ID         string    `json:"id"`
	SenderID   string    `json:"sender_id"`
	ReceiverID string    `json:"receiver_id"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
}

type TypingStatus struct {
	Type     string `json:"type"`
	UserID   string `json:"user_id"`
	Nickname string `json:"nickname"`
	IsTyping bool   `json:"is_typing"`
}


func WSHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()

	userID := r.URL.Query().Get("user_id")
	var user User
	err = database.DB.QueryRow("SELECT id, nickname FROM users WHERE id = ?", userID).Scan(&user.ID, &user.Nickname)
	if err != nil {
		log.Printf("Error fetching user info: %v", err)
		return
	}

	clientsMux.Lock()
	clients[userID] = conn
	users[userID] = user
	clientsMux.Unlock()

	broadcastOnlineUsers()

	defer func() {
		clientsMux.Lock()
		delete(clients, userID)
		delete(users, userID)
		clientsMux.Unlock()
		broadcastOnlineUsers()
	}()

	for {
		var msg struct {
			Type       string `json:"type"`
			ReceiverID string `json:"receiver_id"`
			Content    string `json:"content"`
			IsTyping   bool   `json:"is_typing"`
		}
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Printf("error: %v", err)
			break
		}


		
		if msg.Type == "message" {
			sendMessage(userID, msg.ReceiverID, msg.Content)
		} else if msg.Type == "typing" {
			sendTypingStatus(userID, msg.ReceiverID, msg.IsTyping)
		}
	}
}

func broadcastOnlineUsers() {
	onlineUsers := make([]User, 0, len(clients))
	clientsMux.Lock()
	for _, user := range users {
		onlineUsers = append(onlineUsers, user)
	}
	clientsMux.Unlock()

	message := struct {
		Type  string `json:"type"`
		Users []User `json:"users"`
	}{
		Type:  "online_users",
		Users: onlineUsers,
	}

	clientsMux.Lock()
	for _, conn := range clients {
		conn.WriteJSON(message)
	}
	clientsMux.Unlock()
}

func GetUsersHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := database.DB.Query("SELECT id, nickname FROM users")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		rows.Scan(&user.ID, &user.Nickname)
		users = append(users, user)
	}

	sort.Slice(users, func(i, j int) bool {
		return users[i].Nickname < users[j].Nickname
	})

	json.NewEncoder(w).Encode(users)
}

func GetMessagesHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	otherUserID := r.URL.Query().Get("other_user_id")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize := 10
	offset := page * pageSize

	rows, err := database.DB.Query(`
		SELECT id, sender_id, receiver_id, content, created_at
		FROM messages
		WHERE (sender_id = ? AND receiver_id = ?) OR (sender_id = ? AND receiver_id = ?)
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, userID, otherUserID, otherUserID, userID, pageSize, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		rows.Scan(&msg.ID, &msg.SenderID, &msg.ReceiverID, &msg.Content, &msg.CreatedAt)
		messages = append(messages, msg)
	}

	// Reverse the order of messages so that the latest message is last
	for i := 0; i < len(messages)/2; i++ {
		j := len(messages) - 1 - i
		messages[i], messages[j] = messages[j], messages[i]
	}

	json.NewEncoder(w).Encode(messages)
}

func sendMessage(senderID, receiverID, content string) {
	msg := Message{
		ID:         uuid.New().String(),
		SenderID:   senderID,
		
		ReceiverID: receiverID,
		Content:    content,
		CreatedAt:  time.Now(),
	}

	_, err := database.DB.Exec("INSERT INTO messages (id, sender_id, receiver_id, content, created_at) VALUES (?, ?, ?, ?, ?)",
		msg.ID, msg.SenderID, msg.ReceiverID, msg.Content, msg.CreatedAt)
	if err != nil {
		log.Printf("Error saving message: %v", err)
		return
	}

	clientsMux.Lock()
	for _, userID := range []string{senderID, receiverID} {
		if conn, ok := clients[userID]; ok {
			conn.WriteJSON(msg)
		}
	}
	clientsMux.Unlock()
}

func sendTypingStatus(senderID, receiverID string, isTyping bool) {
	log.Printf("%s - Sending typing status: sender=%s, receiver=%s, isTyping=%v",
		time.Now().Format("2006/01/02 15:04:05"),
		senderID, receiverID, isTyping)

	clientsMux.Lock()
	sender, ok := users[senderID]
	if !ok {
		clientsMux.Unlock()
		return
	}

	status := TypingStatus{
		Type:     "typing_status",
		UserID:   senderID,
		Nickname: sender.Nickname,
		IsTyping: isTyping,
	}

	// Send typing status to the receiver
	if conn, ok := clients[receiverID]; ok {
		err := conn.WriteJSON(status)
		if err != nil {
			log.Printf("Error sending typing status: %v", err)
		}
	}
	clientsMux.Unlock()
}
