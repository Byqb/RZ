package handlers

import (
	"net/http"
	"time"
	"github.com/google/uuid"
	"encoding/json"
	"log"
	"realtime-chat/database"
	"strings"
)


type Post struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	Title      string    `json:"title"`
	Content    string    `json:"content"`
	Categories []string  `json:"categories"`
	CreatedAt  time.Time `json:"created_at"`
}

func CreatePostHandler(w http.ResponseWriter, r *http.Request) {
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
	_, err = database.DB.Exec("INSERT INTO posts (id, user_id, title, content, categories, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		post.ID, post.UserID, post.Title, post.Content, strings.Join(post.Categories, ","), post.CreatedAt)
	if err != nil {
		log.Printf("Error inserting post into database: %v", err)
		http.Error(w, "Error creating post", http.StatusInternalServerError)
		return
	}

	// Fetch user nickname
	var userNickname string
	err = database.DB.QueryRow("SELECT nickname FROM users WHERE id = ?", userID).Scan(&userNickname)
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



func GetPostsHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := database.DB.Query(`
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
