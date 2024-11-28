package handlers

import (
	"net/http"
	"time"
	"github.com/google/uuid"
	"fmt"
	"encoding/json"
	"regexp"
	"golang.org/x/crypto/bcrypt"
	"realtime-chat/database"
)


func RegisterHandler(w http.ResponseWriter, r *http.Request) {
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

	_, err = database.DB.Exec("INSERT INTO users (id, nickname, age, gender, first_name, last_name, email, password) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		uuid.New().String(), user.Nickname, user.Age, user.Gender, user.FirstName, user.LastName, user.Email, hashedPassword)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	CreateSession(w, user.Nickname)
	w.WriteHeader(http.StatusCreated)
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
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
	err := database.DB.QueryRow("SELECT id, nickname, first_name, last_name, password FROM users WHERE email = ? OR nickname = ?", 
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

func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	// Set the cookie expiration date to a time in the past
	DestroySession(w, r)
	http.Redirect(w, r, "/", http.StatusSeeOther)

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
