package main

import (
	"net/http"
	"log"
	"os"

	"realtime-chat/database"
	"realtime-chat/handlers"
	)






func main() {
	database.InitDB()
	defer database.DB.Close()

	http.HandleFunc("/register", handlers.RegisterHandler)
	http.HandleFunc("/login", handlers.LoginHandler)
	http.HandleFunc("/logout", handlers.LogoutHandler)
	http.HandleFunc("/ws", handlers.WSHandler)
	http.HandleFunc("/get-messages", handlers.GetMessagesHandler)
	http.HandleFunc("/get-users", handlers.GetUsersHandler)
	http.HandleFunc("/create-post", handlers.CreatePostHandler)
	http.HandleFunc("/get-posts", handlers.GetPostsHandler)
	http.HandleFunc("/create-comment", handlers.CreateCommentHandler)
	http.HandleFunc("/get-comments", handlers.GetCommentsHandler)

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