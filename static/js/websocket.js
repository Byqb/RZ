function initWebSocket() {
    socket = new WebSocket(`ws://${window.location.host}/ws?user_id=${currentUser.id}`);
    socket.onmessage = handleWebSocketMessage;
}

//TODO: The post should be handled here so it can be live 
function handleWebSocketMessage(event) {
    const data = JSON.parse(event.data);
    console.log('Received WebSocket message:', data);

    if (data.type === 'online_users') {
        onlineUsers = new Set(data.users.map(user => user.id));
        updateUserList();
    } else if (data.type === 'typing_status') {
        handleTypingStatus(data);
    } else if (data.sender_id === currentChatUser || data.receiver_id === currentChatUser) {
        // If chat is currently open with this user, display message without notification
        displayMessage(data);
    } else if (data.sender_id !== currentUser.id) {
        // Only increment unread messages if we're not currently chatting with this user
        // or if the chat container is hidden
        if (data.sender_id !== currentChatUser || chatContainer.style.display === 'none') {
            incrementUnreadMessages(data.sender_id);
            updateUserList();
        }
    }
    fetchUsers();
}

function handleTypingStatus(data) {
    console.log('Handling typing status:', data); // Debug log
    // Update user list typing indicator
    const userElement = userList.querySelector(`[data-user-id="${data.user_id}"]`);
    if (userElement) {
        const typingIndicator = userElement.querySelector('.typing-indicator');
        if (data.is_typing) {
            typingIndicator.style.display = 'inline';
        } else {
            typingIndicator.style.display = 'none';
        }
    }

    // Update chat area typing indicator
    const typingIndicator = document.getElementById('typing-indicator');
    if (data.user_id === currentChatUser) {
        if (data.is_typing) {
            typingIndicator.textContent = `${data.nickname} is typing...`;
            typingIndicator.style.display = 'block';
        } else {
            typingIndicator.style.display = 'none';
        }
    }
}

function fetchUsers() {
    fetch('/get-users')
        .then(response => response.json())
        .then(users => {
            userList.innerHTML = '';
            users.forEach(user => {
                if (user.id !== currentUser.id) {
                    allUsers[user.id] = user;
                    const userElement = document.createElement('div');
                    userElement.className = 'user-item';
                    userElement.innerHTML = `
                        <span class="user-name">${user.nickname}</span>
                        <span class="notification-badge" style="display: none;"></span>
                        <span class="typing-indicator" style="display: none;">typing...</span>
                    `;
                    userElement.setAttribute('data-user-id', user.id);
                    userElement.onclick = () => loadChat(user.id);

                    userList.appendChild(userElement);
                }
            });
            updateUserList();
        });
}

function updateUserList() {
    const userItems = userList.getElementsByClassName('user-item');
    for (let item of userItems) {
        const userId = item.getAttribute('data-user-id');
        if (onlineUsers.has(userId)) {
            item.classList.add('online'); // Add the online class
        } else {
            item.classList.remove('online'); // Remove the online class
        }
        updateNotificationBadge(item, userId);
    }
}

function updateNotificationBadge(userElement, userId) {
    const badge = userElement.querySelector('.notification-badge');
    const unreadCount = unreadMessages[userId] || 0;
    if (unreadCount > 0) {
        badge.style.display = 'inline';
        badge.textContent = unreadCount;
    } else {
        badge.style.display = 'none';
    }
}

function incrementUnreadMessages(userId) {
    if (!unreadMessages[userId]) {
        unreadMessages[userId] = 0;
    }
    unreadMessages[userId]++;
}

function loadChat(userId) {
    currentChatUser = userId;
    chatMessages.innerHTML = ''; // Clear current messages
    currentPage = 0;
    hasMoreMessages = true;

    // Clear unread messages for this user when opening their chat
    unreadMessages[userId] = 0;
    updateUserList(); // Update the UI to remove notification badge

    // Show chat container if it's hidden
    chatContainer.style.display = 'block';
    toggleChatBtn.textContent = 'Close Chat';

    // Check if we have chat history for this user
    if (chatHistory[userId]) {
        chatHistory[userId].forEach(message => {
            const messageElement = createMessageElement(message);
            chatMessages.appendChild(messageElement);
        });
    } else {
        // If no history, fetch messages from the server
        fetchMessages();
    }

    const chatPartner = allUsers[userId];
    document.getElementById('chat-title').textContent = `Chat with ${chatPartner ? chatPartner.nickname : 'Unknown User'}`;
    // Reset typing indicator when changing chats
    document.getElementById('typing-indicator').style.display = 'none';

}

// Define the function to send a welcome message
function sendWelcomeMessage() {
    const welcomeMessage = {
        sender_id: 'system', // Assuming 'system' or another identifier for system messages
        content: "Welcome to the chat room! Start the conversation by sending a message.",
        created_at: new Date().toISOString()
    };

    // Display the welcome message in the chat
    displayMessage(welcomeMessage);
}

function fetchMessages() {
    if (!hasMoreMessages || isLoadingMoreMessages) return Promise.resolve();

    isLoadingMoreMessages = true;
    return fetch(`/get-messages?user_id=${currentUser.id}&other_user_id=${currentChatUser}&page=${currentPage}`)
        .then(response => response.json())
        .then(messages => {
            if (!messages || messages.length === 0) {
                // If no messages are found and it's the first page, send a welcome message
                if (currentPage === 0) {
                    sendWelcomeMessage();
                }
                hasMoreMessages = false; // No more messages to fetch
                isLoadingMoreMessages = false;
                return;
            }

            if (messages.length < 70) {
                hasMoreMessages = false;
            }

            const fragment = document.createDocumentFragment();
            messages.forEach(message => {
                const messageElement = createMessageElement(message);
                fragment.appendChild(messageElement);
            });

            const scrollHeightBefore = chatMessages.scrollHeight;
            chatMessages.insertBefore(fragment, chatMessages.firstChild);

            if (currentPage === 0) {
                chatMessages.scrollTop = chatMessages.scrollHeight;
            } else {
                chatMessages.scrollTop = chatMessages.scrollHeight - scrollHeightBefore;
            }
            currentPage++;
            isLoadingMoreMessages = false;
        });
}

function createMessageElement(message) {

    const escapeHTML = (str) => {
        const div = document.createElement('div');
        div.textContent = str;
        return div.innerHTML;
    };

    const messageElement = document.createElement('div');
    messageElement.className = 'message';
    const sender = message.sender_id === currentUser.id ? 'You' : (allUsers[message.sender_id] ? allUsers[message.sender_id].nickname : 'Unknown User');
    messageElement.innerHTML = `
        <span class="sender">${sender}</span>
        <span class="time">${new Date(message.created_at).toLocaleString()}</span>
        <p>${escapeHTML(message.content)}</p>
    `;
    return messageElement;
}

function handleScroll() {
    if (chatMessages.scrollTop === 0 && hasMoreMessages && !isLoadingMoreMessages) {
        if (scrollTimeout !== null) {
            clearTimeout(scrollTimeout);
        }

        scrollTimeout = setTimeout(() => {
            const scrollHeightBefore = chatMessages.scrollHeight;
            fetchMessages().then(() => {
                chatMessages.scrollTop = chatMessages.scrollHeight - scrollHeightBefore;
                scrollTimeout = null;
            });
        }, 250); // Wait for 250ms of inactivity before loading more messages
    }
}

function displayMessage(message) {
    const messageElement = createMessageElement(message);
    chatMessages.appendChild(messageElement);
    chatMessages.scrollTop = chatMessages.scrollHeight;
}

function loadMoreMessages() {
    fetchMessages();
}

function handleEnterKey(e) {
    if (e.key === 'Enter') {
        sendMessage();
    }
}

function sendMessage() {
    const content = messageInput.value.trim();
    if (content && currentChatUser) {
        // Check character count instead of word count
        if (content.length > 70) { // Limiting to 70 characters
            alert('Message cannot be longer than 70 characters!');
            return;
        }

        const message = {
            sender_id: currentUser.id,
            receiver_id: currentChatUser,
            content: content,
            created_at: new Date().toISOString()
        };

        // Store the message in chat history
        if (!chatHistory[currentChatUser]) {
            chatHistory[currentChatUser] = [];
        }
        chatHistory[currentChatUser].push(message);

        socket.send(JSON.stringify({
            type: 'message',
            receiver_id: currentChatUser,
            content: content
        }));
        messageInput.value = '';
    }
}

function handleTyping() {
    console.log('Handling typing event'); // Debug log
    if (currentChatUser) {
        clearTimeout(typingTimeout);
        socket.send(JSON.stringify({
            type: 'typing',
            receiver_id: currentChatUser,
            is_typing: true
        }));
        console.log('Sent typing true'); // Debug log
        typingTimeout = setTimeout(() => {
            socket.send(JSON.stringify({
                type: 'typing',
                receiver_id: currentChatUser,
                is_typing: false
            }));
            console.log('Sent typing false'); // Debug log
        }, 1000);
    }
}

  // Update the real-time counter to count characters
  messageInput.addEventListener('input', function() {
    const content = this.value;
    const charCount = content.length;
    const remainingChars = 70 - charCount; // 10 character limit
    
    // Update the UI to show remaining characters
    const charCounter = document.createElement('div');
    charCounter.id = 'word-counter'; // keeping the same ID for consistency
    charCounter.style.fontSize = '12px';
    charCounter.style.color = remainingChars >= 0 ? '#666' : 'red';
    charCounter.textContent = `${remainingChars} characters remaining`;
    
    // Remove existing counter if present
    const existingCounter = document.getElementById('word-counter');
    if (existingCounter) {
        existingCounter.remove();
    }
    
    // Add the new counter
    messageInput.parentElement.appendChild(charCounter);
});

// Helper function to update character counters
function updateCharCounter(element, remainingChars, counterId) {
    const counter = document.createElement('div');
    counter.id = counterId;
    counter.style.fontSize = '12px';
    counter.style.color = remainingChars >= 0 ? '#666' : 'red';
    counter.textContent = `${remainingChars} characters remaining`;
    
    // Remove existing counter if present
    const existingCounter = document.getElementById(counterId);
    if (existingCounter) {
        existingCounter.remove();
    }
    
    // Add the new counter
    element.parentElement.appendChild(counter);
}