package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
"os"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	_ "github.com/mattn/go-sqlite3"
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
		age INTEGER,
		gender TEXT,
		first_name TEXT,
		last_name TEXT,
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

var sessions = map[string]string{} // key is the session ID, value is the user ID
var userSessions = map[string]string{}
func CreateSession(w http.ResponseWriter, userID string) {
    // Generate a new UUID for the session ID
    sessionID := uuid.NewString()
    if existingSessionID, exists := userSessions[userID]; exists {
        delete(sessions, existingSessionID)
		http.SetCookie(w, &http.Cookie{
			Name:     "session_id",
			Value:    "",
			Expires:  time.Now().Add(-1 * time.Hour), // Expire the old session cookie immediately
			HttpOnly: true,
		})
    }
    
    // Store the session ID and associated userID in the session store
    sessions[sessionID] = userID
    userSessions[userID] = sessionID
    // Set a cookie with the session ID
    http.SetCookie(w, &http.Cookie{
        Name:     "session_id",
        Value:    sessionID,
        Expires:  time.Now().Add(24 * time.Hour), // Set the expiration to 24 hours
        HttpOnly: true,                           // Make it inaccessible via JavaScript
    })
}
func GetUserIDFromSession(r *http.Request) (string, bool) {
    cookie, err := r.Cookie("session_id")
    if err != nil {
        return "", false
    }
    
    // Check if session ID exists in the session store
    userID, exists := sessions[cookie.Value]
    if !exists {
        return "", false
    }
    return userID, true
}
// after the user log we delete 
func DestroySession(w http.ResponseWriter, r *http.Request) {
    cookie, err := r.Cookie("session_id")
    if err != nil {
        return
    }
    userID, exists := sessions[cookie.Value]
	if !exists {
		return
	}
    // Delete session from the session store
    delete(sessions, cookie.Value)
    delete(userSessions, userID)
    // Expire the cookie
    http.SetCookie(w, &http.Cookie{
        Name:     "session_id",
        Value:    "",
        Expires:  time.Now().Add(-1 * time.Hour), // Expire the cookie immediately
        HttpOnly: true,
    })
}



// FIXME: fix the information for the user
func registerHandler(w http.ResponseWriter, r *http.Request) {
	var user struct {
		Nickname  string `json:"nickname"`
		Age       int    `json:"age"`
		Gender    string `json:"gender"`
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
		Email     string `json:"email"`
		Password  string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate email format
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(user.Email) {
		http.Error(w, "Invalid email format", http.StatusBadRequest)
		return
	}

	if len(user.Password) < 8 {
		fmt.Println("Password must be at least 8 characters long.")
		return
	}

	// Check for at least one uppercase letter
	if !regexp.MustCompile(`[A-Z]`).MatchString(user.Password) {
		fmt.Println("Password must contain at least one uppercase letter.")
		return
	}

	// Check for at least one lowercase letter
	if !regexp.MustCompile(`[a-z]`).MatchString(user.Password) {
		fmt.Println("Password must contain at least one lowercase letter.")
		return
	}

	// Check for at least one digit
	if !regexp.MustCompile(`\d`).MatchString(user.Password) {
		fmt.Println("Password must contain at least one number.")
		return
	}

	// Check for at least one special character
	if !regexp.MustCompile(`[@$!%*?&]`).MatchString(user.Password) {
		fmt.Println("Password must contain at least one special character.")
		return
	}


	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Error hashing password", http.StatusInternalServerError)
		return
	}

	_, err = db.Exec("INSERT INTO users (id, nickname, age, gender, first_name, last_name, email, password) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		uuid.New().String(), user.Nickname, user.Age, user.Gender, user.FirstName, user.LastName, user.Email, hashedPassword)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	CreateSession(w, user.Nickname)
	w.WriteHeader(http.StatusCreated)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	var creds struct {
		Identifier string `json:"identifier"`
		Password   string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var user struct {
		ID        string `json:"id"`
		Nickname  string `json:"nickname"`
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
	}
	var hashedPassword string

	// Updated query to include first_name and last_name
	err := db.QueryRow("SELECT id, nickname, first_name, last_name, password FROM users WHERE email = ? OR nickname = ?", 
		creds.Identifier, creds.Identifier).Scan(&user.ID, &user.Nickname, &user.FirstName, &user.LastName, &hashedPassword)
	if err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(creds.Password)); err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	CreateSession(w, user.Nickname)
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

		// // log.Printf("Received message: %+v", msg) // Debug log

		// if isNewChat(userID, msg.ReceiverID) {

		// 	fmt.Println("New chat detected") // Debug log

		// 	welcomeMessage := Message{
		// 		ID:         uuid.New().String(),
		// 		SenderID:   "system",
		// 		ReceiverID: msg.ReceiverID,
		// 		Content:    "Welcome to your new chat room!",
		// 		CreatedAt:  time.Now(),
		// 	}

		// 	// Insert the welcome message into the database
		// 	_, err := db.Exec("INSERT INTO messages (id, sender_id, receiver_id, content, created_at) VALUES (?, ?, ?, ?, ?)",
		// 		welcomeMessage.ID, welcomeMessage.SenderID, welcomeMessage.ReceiverID, welcomeMessage.Content, welcomeMessage.CreatedAt)
		// 	if err != nil {
		// 		log.Printf("Error saving welcome message: %v", err)
		// 	}

		// 	// Send the welcome message to both users
		// 	clientsMux.Lock()
		// 	for _, userID := range []string{userID, msg.ReceiverID} {
		// 		if conn, ok := clients[userID]; ok {
		// 			conn.WriteJSON(welcomeMessage)
		// 		}
		// 	}
		// 	clientsMux.Unlock()

		// }

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

// func isNewChat(senderID, receiverID string) bool {
// 	var count int
// 	err := db.QueryRow(`
// 		SELECT COUNT(*) FROM messages
// 		WHERE (sender_id = ? AND receiver_id = ?) OR (sender_id = ? AND receiver_id = ?)
// 	`, senderID, receiverID, receiverID, senderID).Scan(&count)
// 	if err != nil {
// 		log.Printf("Error checking existing chat: %v", err)
// 		return false
// 	}
// 	fmt.Println(count) // Debug log
// 	return count == 0
// }

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

	// Send typing status to the receiver
	if conn, ok := clients[receiverID]; ok {
		err := conn.WriteJSON(status)
		if err != nil {
			log.Printf("Error sending typing status: %v", err)
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

	// Fetch user nickname
	var userNickname string
	err = db.QueryRow("SELECT nickname FROM users WHERE id = ?", userID).Scan(&userNickname)
	if err != nil {
		log.Printf("Error fetching user nickname: %v", err)
		http.Error(w, "Error creating post", http.StatusInternalServerError)
		return
	}

	// Prepare response
	type PostResponse struct {
		Post
		UserNickname string `json:"user_nickname"`
	}

	response := PostResponse{
		Post:         post,
		UserNickname: userNickname,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func getPostsHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(`
		SELECT p.id, p.user_id, u.nickname, p.title, p.content, p.categories, p.created_at 
		FROM posts p
		JOIN users u ON p.user_id = u.id
		ORDER BY p.created_at DESC
	`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type PostWithUser struct {
		Post
		UserNickname string `json:"user_nickname"`
	}

	var posts []PostWithUser
	for rows.Next() {
		var post PostWithUser
		var categoriesString string
		err := rows.Scan(&post.ID, &post.UserID, &post.UserNickname, &post.Title, &post.Content, &categoriesString, &post.CreatedAt)
		if err != nil {
			log.Printf("Error scanning post row: %v", err)
			continue
		}
		post.Categories = strings.Split(categoriesString, ",")
		posts = append(posts, post)
	}

	w.Header().Set("Content-Type", "application/json")
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
	err := json.NewDecoder(r.Body).Decode(&comment)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	comment.ID = uuid.New().String()
	comment.UserID = userID
	comment.CreatedAt = time.Now()

	// Insert comment into database
	_, err = db.Exec("INSERT INTO comments (id, post_id, user_id, content, created_at) VALUES (?, ?, ?, ?, ?)",
		comment.ID, comment.PostID, comment.UserID, comment.Content, comment.CreatedAt)
	if err != nil {
		log.Printf("Error inserting comment into database: %v", err)
		http.Error(w, "Error creating comment", http.StatusInternalServerError)
		return
	}

	// Fetch user nickname
	var userNickname string
	err = db.QueryRow("SELECT nickname FROM users WHERE id = ?", userID).Scan(&userNickname)
	if err != nil {
		log.Printf("Error fetching user nickname: %v", err)
		http.Error(w, "Error creating comment", http.StatusInternalServerError)
		return
	}

	// Prepare response
	type CommentResponse struct {
		Comment
		UserNickname string `json:"user_nickname"`
	}

	response := CommentResponse{
		Comment:      comment,
		UserNickname: userNickname,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func getCommentsHandler(w http.ResponseWriter, r *http.Request) {
	postID := r.URL.Query().Get("post_id")
	rows, err := db.Query(`
		SELECT c.id, c.post_id, c.user_id, u.nickname, c.content, c.created_at 
		FROM comments c
		JOIN users u ON c.user_id = u.id
		WHERE c.post_id = ? 
		ORDER BY c.created_at
	`, postID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type CommentWithUser struct {
		Comment
		UserNickname string `json:"user_nickname"`
	}

	var comments []CommentWithUser
	for rows.Next() {
		var comment CommentWithUser
		err := rows.Scan(&comment.ID, &comment.PostID, &comment.UserID, &comment.UserNickname, &comment.Content, &comment.CreatedAt)
		if err != nil {
			log.Printf("Error scanning comment row: %v", err)
			continue
		}
		comments = append(comments, comment)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(comments)
}

// TODO: Implement cookie handling
// func setCookieHandler(w http.ResponseWriter, r *http.Request) {
// 	expiration := time.Now().Add(24 * time.Hour)
// 	cookie := http.Cookie{Name: "username", Value: "john_doe", Expires: expiration}
// 	http.SetCookie(w, &cookie)
// 	w.Write([]byte("Cookie set"))
// }

// TODO: Implement cookie handling
// func getCookieHandler(w http.ResponseWriter, r *http.Request) {
// 	cookie, err := r.Cookie("username")
// 	if err != nil {
// 		w.Write([]byte("Cookie not found"))
// 		return
// 	}
// 	w.Write([]byte("Cookie value: " + cookie.Value))
// }

// TODO: Implement cookie handling to logout
func logoutHandler(w http.ResponseWriter, r *http.Request) {
	// Set the cookie expiration date to a time in the past
	DestroySession(w, r)
	http.Redirect(w, r, "/", http.StatusSeeOther)

}

func main() {
	initDB()
	defer db.Close()

	http.HandleFunc("/register", registerHandler)
	http.HandleFunc("/login", loginHandler)

	http.HandleFunc("/logout", logoutHandler) // Added logout handler
	http.HandleFunc("/ws", wsHandler)
	http.HandleFunc("/get-messages", getMessagesHandler)
	http.HandleFunc("/get-users", getUsersHandler)
	http.HandleFunc("/create-post", createPostHandler)
	http.HandleFunc("/get-posts", getPostsHandler)
	http.HandleFunc("/create-comment", createCommentHandler)
	http.HandleFunc("/get-comments", getCommentsHandler)

	http.HandleFunc("/", customFileServerHandler)


	// let it to work with the ip address
	log.Println("Server is running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
func customFileServerHandler(w http.ResponseWriter, r *http.Request) {
	// Path to the directory containing static files
	indexPath := "./static/index.html"

	// Check if the index.html file exists
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
			// index.html does not exist, send HTTP 500
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
	} else {
		  // index.html exists, serve it using the FileServer
      fs := http.FileServer(http.Dir("./static/"))
      fs.ServeHTTP(w, r)
      return
	}

	// Serve the file using ServeFile
	http.ServeFile(w, r, indexPath)
}
// in reg the email has a problem (finsh)
// handel password space (finsh)
// handel message sand post space and the html (finsh)

// handel the long message and post also accessthe user to input a new line (use the text-area insted of input) (in CSS & MTML) (finsh)
// The post & comments are not live (you need to refresh the page to see the new post)
// error handling to be in the same page (finsh)
// add the seshen  and cookie (finsh)