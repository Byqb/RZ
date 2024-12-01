 // Add these constants at the top of your script
        const TITLE_CHAR_LIMIT = 100;
        const POST_CONTENT_CHAR_LIMIT = 100;
        const COMMENT_CHAR_LIMIT = 100;

        // Add character counter for post title
        document.getElementById('post-title').addEventListener('input', function() {
            const content = this.value;
            const charCount = content.length;
            const remainingChars = TITLE_CHAR_LIMIT - charCount;
            
            updateCharCounter(this, remainingChars, 'title-counter');
        });

        // Add character counter for post content
        document.getElementById('post-content').addEventListener('input', function() {
            const content = this.value;
            const charCount = content.length;
            const remainingChars = POST_CONTENT_CHAR_LIMIT - charCount;
            
            updateCharCounter(this, remainingChars, 'content-counter');
        });

        // Update the createPost function
        function createPost() {
            const title = document.getElementById('post-title').value;
            const content = document.getElementById('post-content').value;
            const categories = Array.from(document.querySelectorAll('.category-option input:checked')).map(input => input.value);

            if (title.length > TITLE_CHAR_LIMIT) {
                alert(`Title cannot be longer than ${TITLE_CHAR_LIMIT} characters!`);
                return;
            }

            if (content.length > POST_CONTENT_CHAR_LIMIT) {
                alert(`Post content cannot be longer than ${POST_CONTENT_CHAR_LIMIT} characters!`);
                return;
            }

            console.log('Attempting to create post:', { title, content, categories });

            const onlySpacesRegex = /^[\s]*$/;
            if (onlySpacesRegex.test(title) || onlySpacesRegex.test(content)) {
                alert('Please enter a valid title and content.');
                return;
            }

            if (!title || !content || categories.length === 0) {
                alert('Please fill in all fields and select at least one category.');
                return;
            }

            if (!currentUser || !currentUser.id) {
                console.error('User not logged in or user ID missing');
                alert('You must be logged in to create a post.');
                return;
            }

            fetch('/create-post', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'User-ID': currentUser.id
                },
                body: JSON.stringify({ title, content, categories })
            }).then(response => {
                console.log('Response status:', response.status);
                if (!response.ok) {
                    return response.text().then(text => {
                        throw new Error(`HTTP error! status: ${response.status}, message: ${text}`);
                    });
                }
                return response.json();
            })
                .then(post => {
                    console.log('Post created successfully:', post);
                    document.getElementById('post-title').value = '';
                    document.getElementById('post-content').value = '';
                    document.querySelectorAll('.category-option input:checked').forEach(input => input.checked = false);
                    addPostToList(post, false); // Add new post to the top
                }).catch(error => {
                    console.error('Error creating post:', error);
                    alert('Error creating post: ' + error.message);
                });
        }

        // Update the addPostToList function to add character counter for comments
        function addPostToList(post, addToBottom = false) {
            const postsContainer = document.getElementById('posts');
            const postElement = document.createElement('div');
            postElement.className = 'post';

            // Create a safe way to handle user inputs
            const escapeHTML = (str) => {
                const div = document.createElement('div');
                div.textContent = str;
                return div.innerHTML;
            };

            postElement.innerHTML = `
    <h4>${escapeHTML(post.title)}</h4>
    <p>${escapeHTML(post.content)}</p>
    <div class="categories">Categories: ${post.categories.map(escapeHTML).join(', ')}</div>
    <div class="author">Posted by: ${escapeHTML(post.user_nickname || currentUser.nickname)}</div>
    <button class="view-comments" data-post-id="${post.id}" style="font-family: 'Press Start 2P', cursive; color: aliceblue;">View Comments</button>
    <div class="comment-section" style="display: none;">
        ${isLoggedIn ? `
            <div class="comment-form">
                <div class="input-container">
                    <input type="text" class="comment-input" 
                        style="font-family: 'Press Start 2P', cursive;" 
                        placeholder="Add a comment(max 100)">
                </div>
                <button class="submit-comment" data-post-id="${post.id}" 
                    style="font-family: 'Press Start 2P', cursive; color: aliceblue;">Submit</button>
            </div>
        ` : ''}
        <div class="comments"></div>
    </div>
`;

            if (addToBottom) {
                postsContainer.appendChild(postElement);
            } else {
                postsContainer.insertBefore(postElement, postsContainer.firstChild);
            }

            const viewCommentsBtn = postElement.querySelector('.view-comments');
            const commentSection = postElement.querySelector('.comment-section');

            viewCommentsBtn.addEventListener('click', () => {
                if (commentSection.style.display === 'none') {
                    fetchComments(post.id, commentSection.querySelector('.comments'));
                    commentSection.style.display = 'block';
                    viewCommentsBtn.textContent = 'Hide Comments';
                } else {
                    commentSection.style.display = 'none';
                    viewCommentsBtn.textContent = 'View Comments';
                }
            });

            if (isLoggedIn) {
                const submitCommentBtn = postElement.querySelector('.submit-comment');
                const commentInput = postElement.querySelector('.comment-input');
                submitCommentBtn.addEventListener('click', () => {
                    createComment(post.id, commentInput.value);
                });
                commentInput.addEventListener('keypress', (e) => {
                    if (e.key === 'Enter') {
                        createComment(post.id, commentInput.value);
                    }
                });
                commentInput.addEventListener('input', function() {
                    const content = this.value;
                    const charCount = content.length;
                    const remainingChars = COMMENT_CHAR_LIMIT - charCount;
                    
                    updateCharCounter(this, remainingChars, `comment-counter-${post.id}`);
                });
            }
        }

        function fetchComments(postId, commentsContainer) {
            fetch(`/get-comments?post_id=${postId}`)
                .then(response => response.json())
                .then(comments => {
                    commentsContainer.innerHTML = '';
                    comments.forEach(comment => {

                        const escapeHTML = (str) => {
                            const div = document.createElement('div');
                            div.textContent = str;
                            return div.innerHTML;
                        };

                        const commentElement = document.createElement('div');
                        commentElement.className = 'comment';
                        commentElement.innerHTML = `
                            <div class="comment-content">${escapeHTML(comment.content)}</div>
                            <div class="comment-meta">
                                <span class="comment-author">${escapeHTML(comment.user_nickname)}</span>
                                <span class="comment-time">${new Date(comment.created_at).toLocaleString()}</span>
                            </div>
                        `;
                        commentsContainer.appendChild(commentElement);
                    });
                })
                .catch(error => console.error('Error fetching comments:', error));
        }

        function createComment(postId, content) {
            if (!content.trim()) {
                alert('Please enter a comment.');
                return;
            }

            if (content.length > COMMENT_CHAR_LIMIT) {
                alert(`Comment cannot be longer than ${COMMENT_CHAR_LIMIT} characters!`);
                return;
            }

            fetch('/create-comment', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'User-ID': currentUser.id
                },
                body: JSON.stringify({ post_id: postId, content })
            }).then(response => {
                if (!response.ok) {
                    throw new Error('Failed to create comment');
                }
                return response.json();
            })
                .then(comment => {
                    const postElement = document.querySelector(`.post button[data-post-id="${postId}"]`).closest('.post');
                    const commentsContainer = postElement.querySelector('.comments');

                    const escapeHTML = (str) => {
                        const div = document.createElement('div');
                        div.textContent = str;
                        return div.innerHTML;
                    };

                    // Add the new comment to the existing comments
                    const commentElement = document.createElement('div');
                    commentElement.className = 'comment';
                    commentElement.innerHTML = `
                    <div class="comment-content">${escapeHTML(comment.content)}</div>
                    <div class="comment-meta">
                        <span class="comment-author">${escapeHTML(comment.user_nickname)}</span>
                        <span class="comment-time">${new Date(comment.created_at).toLocaleString()}</span>
                    </div>
                `;
                    commentsContainer.appendChild(commentElement);

                    // Clear the comment input
                    postElement.querySelector('.comment-input').value = '';
                }).catch(error => {
                    console.error('Error creating comment:', error);
                    alert('Error creating comment. Please try again.');
                });
        }

        function fetchPosts() {
            fetch('/get-posts')
                .then(response => response.json())
                .then(posts => {
                    const postsContainer = document.getElementById('posts');
                    postsContainer.innerHTML = ''; // Clear existing posts
                    posts.forEach(post => addPostToList(post, true)); // Add 'true' to indicate these are existing posts
                })
                .catch(error => console.error('Error fetching posts:', error));
        }



        // Initial display
        authSection.style.display = isLoggedIn ? 'none' : 'block';
        forumSection.style.display = isLoggedIn ? 'block' : 'none';
        document.getElementById('post-creation-area').style.display = isLoggedIn ? 'block' : 'none';

        // If the user is already logged in (e.g., page refresh), fetch posts
        if (isLoggedIn) {
            fetchPosts();
        }

        function forcePageRefresh() {
            location.reload();
        }

      