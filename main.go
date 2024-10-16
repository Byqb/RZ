package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"sort"
		"sync"
	"time"
	"strconv"
	"strings"

	"github.com/gorilla/websocket"
	_ "github.com/mattn/go-sqlite3"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var db *sql.DB
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
	ID        string    `json:"id"`
	SenderID  string    `json:"sender_id"`
	ReceiverID string   `json:"receiver_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type TypingStatus struct {
	Type     string `json:"type"`
	UserID   string `json:"user_id"`
	Nickname string `json:"nickname"`
	IsTyping bool   `json:"is_typing"`
}

type Post struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	Title      string    `json:"title"`
	Content    string    `json:"content"`
	Categories []string  `json:"categories"`
	CreatedAt  time.Time `json:"created_at"`
}

type Comment struct {
	ID        string    `json:"id"`
	PostID    string    `json:"post_id"`
	UserID    string    `json:"user_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

func initDB() {
	var err error
	db, err = sql.Open("sqlite3", "./forum.db")
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			nickname TEXT UNIQUE,
			email TEXT UNIQUE,
			password TEXT
		);
		CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			sender_id TEXT,
			receiver_id TEXT,
			content TEXT,
			created_at DATETIME,
			FOREIGN KEY (sender_id) REFERENCES users(id),
			FOREIGN KEY (receiver_id) REFERENCES users(id)
		);
		CREATE TABLE IF NOT EXISTS posts (
			id TEXT PRIMARY KEY,
			user_id TEXT,
			title TEXT,
			content TEXT,
			categories TEXT,
			created_at DATETIME,
			FOREIGN KEY (user_id) REFERENCES users(id)
		);
		CREATE TABLE IF NOT EXISTS comments (
			id TEXT PRIMARY KEY,
			post_id TEXT,
			user_id TEXT,
			content TEXT,
			created_at DATETIME,
			FOREIGN KEY (post_id) REFERENCES posts(id),
			FOREIGN KEY (user_id) REFERENCES users(id)
		);
	`)
	if err != nil {
		log.Fatal(err)
	}
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	var user struct {
		Nickname string `json:"nickname"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	json.NewDecoder(r.Body).Decode(&user)

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	_, err := db.Exec("INSERT INTO users (id, nickname, email, password) VALUES (?, ?, ?, ?)",
		uuid.New().String(), user.Nickname, user.Email, hashedPassword)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	var creds struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	json.NewDecoder(r.Body).Decode(&creds)

	var user User
	var hashedPassword string
	err := db.QueryRow("SELECT id, nickname, password FROM users WHERE email = ?", creds.Email).Scan(&user.ID, &user.Nickname, &hashedPassword)
	if err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(creds.Password)); err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	json.NewEncoder(w).Encode(user)
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()

	userID := r.URL.Query().Get("user_id")
	var user User
	err = db.QueryRow("SELECT id, nickname FROM users WHERE id = ?", userID).Scan(&user.ID, &user.Nickname)
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

		log.Printf("Received message: %+v", msg) // Debug log

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

func getUsersHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, nickname FROM users")
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

func getMessagesHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	otherUserID := r.URL.Query().Get("other_user_id")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize := 10
	offset := page * pageSize

	rows, err := db.Query(`
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

	_, err := db.Exec("INSERT INTO messages (id, sender_id, receiver_id, content, created_at) VALUES (?, ?, ?, ?, ?)",
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

	// Send typing status to all connected clients
	for _, conn := range clients {
		err := conn.WriteJSON(status)
		if err != nil {
			log.Printf("%s - Error sending typing status: %v", 
				time.Now().Format("2006/01/02 15:04:05"), err)
		}
	}
	clientsMux.Unlock()
}

func createPostHandler(w http.ResponseWriter, r *http.Request) {
	// Check if user is authenticated
	userID := r.Header.Get("User-ID")
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var post Post
	err := json.NewDecoder(r.Body).Decode(&post)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	post.ID = uuid.New().String()
	post.UserID = userID
	post.CreatedAt = time.Now()

	// Insert post into database
	_, err = db.Exec("INSERT INTO posts (id, user_id, title, content, categories, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		post.ID, post.UserID, post.Title, post.Content, strings.Join(post.Categories, ","), post.CreatedAt)
	if err != nil {
		log.Printf("Error inserting post into database: %v", err)
		http.Error(w, "Error creating post", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(post)
}

func getPostsHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, user_id, title, content, categories, created_at FROM posts ORDER BY created_at DESC")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var post Post
		var categoriesJSON string
		rows.Scan(&post.ID, &post.UserID, &post.Title, &post.Content, &categoriesJSON, &post.CreatedAt)
		json.Unmarshal([]byte(categoriesJSON), &post.Categories)
		posts = append(posts, post)
	}

	json.NewEncoder(w).Encode(posts)
}

func createCommentHandler(w http.ResponseWriter, r *http.Request) {
	// Check if user is authenticated
	userID := r.Header.Get("User-ID")
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var comment Comment
	json.NewDecoder(r.Body).Decode(&comment)
	comment.ID = uuid.New().String()
	comment.CreatedAt = time.Now()

	_, err := db.Exec("INSERT INTO comments (id, post_id, user_id, content, created_at) VALUES (?, ?, ?, ?, ?)",
		comment.ID, comment.PostID, comment.UserID, comment.Content, comment.CreatedAt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(comment)
}

func getCommentsHandler(w http.ResponseWriter, r *http.Request) {
	postID := r.URL.Query().Get("post_id")
	rows, err := db.Query("SELECT id, post_id, user_id, content, created_at FROM comments WHERE post_id = ? ORDER BY created_at", postID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var comments []Comment
	for rows.Next() {
		var comment Comment
		rows.Scan(&comment.ID, &comment.PostID, &comment.UserID, &comment.Content, &comment.CreatedAt)
		comments = append(comments, comment)
	}

	json.NewEncoder(w).Encode(comments)
}

func main() {
	initDB()
	defer db.Close()

	http.HandleFunc("/register", registerHandler)
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/ws", wsHandler)
	http.HandleFunc("/get-messages", getMessagesHandler)
	http.HandleFunc("/get-users", getUsersHandler)
	http.HandleFunc("/create-post", createPostHandler)
	http.HandleFunc("/get-posts", getPostsHandler)
	http.HandleFunc("/create-comment", createCommentHandler)
	http.HandleFunc("/get-comments", getCommentsHandler)

	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)

	log.Println("Server is running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
