package handlers

import (
	"net/http"
	"time"
	"github.com/google/uuid"
	"encoding/json"
	"log"
	"realtime-chat/database"
)

type Comment struct {
	ID        string    `json:"id"`
	PostID    string    `json:"post_id"`
	UserID    string    `json:"user_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

func CreateCommentHandler(w http.ResponseWriter, r *http.Request) {
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
	_, err = database.DB.Exec("INSERT INTO comments (id, post_id, user_id, content, created_at) VALUES (?, ?, ?, ?, ?)",
		comment.ID, comment.PostID, comment.UserID, comment.Content, comment.CreatedAt)
	if err != nil {
		log.Printf("Error inserting comment into database: %v", err)
		http.Error(w, "Error creating comment", http.StatusInternalServerError)
		return
	}

	// Fetch user nickname
	var userNickname string
	err = database.DB.QueryRow("SELECT nickname FROM users WHERE id = ?", userID).Scan(&userNickname)
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

func GetCommentsHandler(w http.ResponseWriter, r *http.Request) {
	postID := r.URL.Query().Get("post_id")
	rows, err := database.DB.Query(`
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