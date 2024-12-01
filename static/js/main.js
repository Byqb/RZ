 let currentUser = null;
        let socket = null;
        let currentChatUser = null;
        let onlineUsers = new Set();
        let allUsers = {};
        let unreadMessages = {};
        let isLoadingMoreMessages = false;
        let currentPage = 0;
        let hasMoreMessages = true;
        let scrollTimeout = null;
        let typingTimeout = null;
        let currentPost = null;
        let isLoggedIn = false;
        let chatHistory = {};

        const authSection = document.getElementById('auth-section');
        const forumSection = document.getElementById('forum-section');
        const registerForm = document.getElementById('register-form');
        const loginForm = document.getElementById('login-form');
        const userList = document.getElementById('user-list');
        const chatMessages = document.getElementById('chat-messages');
        const messageInput = document.getElementById('message-input');
        const sendMessageBtn = document.getElementById('send-message');
        const toggleChatBtn = document.getElementById('toggle-chat-btn');
        const chatContainer = document.getElementById('chat-container');

        // Check if user data is stored in localStorage
        const storedUser = localStorage.getItem('currentUser');
        if (storedUser) {
            currentUser = JSON.parse(storedUser);
            isLoggedIn = true;
            authSection.style.display = 'none';
            forumSection.style.display = 'block';
            document.getElementById('post-creation-area').style.display = 'block';
            initWebSocket();
            fetchUsers();
            fetchPosts(); // Fetch posts after restoring session
        }

        registerForm.addEventListener('submit', handleRegister);
        loginForm.addEventListener('submit', handleLogin);
        sendMessageBtn.addEventListener('click', sendMessage);
        messageInput.addEventListener('keypress', handleEnterKey);
        chatMessages.addEventListener('scroll', handleScroll);
        document.getElementById('load-more-messages').addEventListener('click', loadMoreMessages);
        messageInput.addEventListener('input', handleTyping);
        document.getElementById('create-post-btn').addEventListener('click', createPost);

        // Add event listener for logout button
        document.getElementById('logout-btn').addEventListener('click', logout);

        toggleChatBtn.addEventListener('click', () => {
            if (chatContainer.style.display === 'none') {
                chatContainer.style.display = 'block'; // Show chat
                toggleChatBtn.textContent = 'Close Chat'; // Change button text
            } else {
                chatContainer.style.display = 'none'; // Hide chat
                toggleChatBtn.textContent = 'Open Chat'; // Change button text
            }
            // forcePageRefresh();  
        });

        function handleRegister(e) {
            e.preventDefault();

            // Retrieve form values
            const nickname = document.getElementById('reg-nickname').value;
            const age = parseInt(document.getElementById('reg-age').value, 10);
            const gender = document.getElementById('reg-gender').value;
            const firstName = document.getElementById('reg-first-name').value;
            const lastName = document.getElementById('reg-last-name').value;
            const email = document.getElementById('reg-email').value;
            const password = document.getElementById('reg-password').value;

            // Simple validation (optional)
            if (!nickname || isNaN(age) || !gender || !firstName || !lastName || !email || !password) {
                alert("Please fill out all fields with valid information.");
                return;
            }

            /* The password testing */
            const passwordRegex = /^(?=.*[A-Z])(?=.*[a-z])(?=.*\d)(?=.*[@$!%*?&])[A-Za-z\d@$!%*?&]{8,}$/;
            if (!passwordRegex.test(password)) {
                alert("Password must be at least 8 characters long and contain at least one uppercase letter, one lowercase letter, one number, and one special character.");
                return;
            }

            const emailRegex = /^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$/;
            if (!emailRegex.test(email)) {
                alert("Please enter a valid email address.");
                return; // Stop the function if email is invalid
            }


            // Send request to server
            fetch('/register', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ nickname, age, gender, firstName, lastName, email, password })
            })
                .then(response => {
                    if (response.ok) {
                        alert('Registration successful. Please log in.');
                    } else {
                        response.text().then(errorMessage => {
                            alert(`Registration failed: ${errorMessage}`);
                        });
                    }
                })
                .catch(error => {
                    console.error('Error:', error);
                    alert('An error occurred. Please try again.');
                });
        }

        function handleLogin(e) {
            e.preventDefault();
            const identifier = document.getElementById('login-identifier').value;
            const password = document.getElementById('login-password').value;

            fetch('/login', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ identifier, password })
            })
                .then(response => {
                    if (!response.ok) {
                        throw new Error('Login failed');
                    }
                    return response.json();
                })
                .then(user => {
                    currentUser = user;
                    isLoggedIn = true;
                    localStorage.setItem('currentUser', JSON.stringify(currentUser));
                    
                    // Show welcome message
                    const welcomeDiv = document.getElementById('welcome-message');
                    welcomeDiv.textContent = `Welcome, ${user.firstName} ${user.lastName}!`;
                    welcomeDiv.style.display = 'block';
                    
                    // Hide welcome message after 5 seconds
                    setTimeout(() => {
                        welcomeDiv.style.opacity = '0';
                        setTimeout(() => {
                            welcomeDiv.style.display = 'none';
                            welcomeDiv.style.opacity = '1';
                        }, 1000);
                    }, 5000);

                    authSection.style.display = 'none';
                    forumSection.style.display = 'block';
                    document.getElementById('post-creation-area').style.display = 'block';
                    initWebSocket();
                    fetchUsers();
                    fetchPosts();
                })
                .catch(error => {
                    playSound();
                    alert(error.message || 'Login failed. Nani!!');
                });
        }

        // Add this function to play the sound
        function playSound() {
            const audio = new Audio('nani.mp3'); // Replace with the actual path to your sound file
            audio.play();
        }

       

        // Add a function to clear user data on logout
        function logout() {
            // Close the WebSocket connection if it exists
            if (socket) {
                socket.close(); // Close the WebSocket connection
            }

            localStorage.removeItem('currentUser');
            currentUser = null;
            isLoggedIn = false;
            authSection.style.display = 'block';
            forumSection.style.display = 'none';
            document.getElementById('post-creation-area').style.display = 'none';
        }