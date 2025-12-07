package store

import (
	"sync"
	"time"

	"onebookai/pkg/domain"
)

// MemoryStore keeps metadata in-process for the MVP.
type MemoryStore struct {
	mu     sync.RWMutex
	books  map[string]domain.Book
	chats  map[string][]domain.Message
	orders []string
	users  map[string]domain.User // key: user ID
	email  map[string]string      // email -> user ID
	sess   map[string]string      // token -> user ID
}

// NewMemoryStore initializes an empty in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		books: make(map[string]domain.Book),
		chats: make(map[string][]domain.Message),
		users: make(map[string]domain.User),
		email: make(map[string]string),
		sess:  make(map[string]string),
	}
}

// SaveBook stores or replaces a book record and tracks insertion order.
func (m *MemoryStore) SaveBook(b domain.Book) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.books[b.ID]; !exists {
		m.orders = append(m.orders, b.ID)
	}
	m.books[b.ID] = b
}

// SetStatus updates status and optional error message.
func (m *MemoryStore) SetStatus(id string, status domain.BookStatus, errMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	book, ok := m.books[id]
	if !ok {
		return
	}
	book.Status = status
	book.ErrorMessage = errMsg
	book.UpdatedAt = time.Now().UTC()
	m.books[id] = book
}

// ListBooks returns books in insertion order (best-effort).
func (m *MemoryStore) ListBooks() []domain.Book {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]domain.Book, 0, len(m.orders))
	for _, id := range m.orders {
		if b, ok := m.books[id]; ok {
			res = append(res, b)
		}
	}
	return res
}

// ListBooksByOwner returns books filtered by owner ID.
func (m *MemoryStore) ListBooksByOwner(ownerID string) []domain.Book {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]domain.Book, 0, len(m.orders))
	for _, id := range m.orders {
		if b, ok := m.books[id]; ok && b.OwnerID == ownerID {
			res = append(res, b)
		}
	}
	return res
}

// GetBook retrieves a book by ID.
func (m *MemoryStore) GetBook(id string) (domain.Book, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	b, ok := m.books[id]
	return b, ok
}

// DeleteBook removes a book and its chat history.
func (m *MemoryStore) DeleteBook(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.books, id)
	delete(m.chats, id)
	filtered := m.orders[:0]
	for _, item := range m.orders {
		if item != id {
			filtered = append(filtered, item)
		}
	}
	m.orders = filtered
}

// AppendMessage records a message linked to a book.
func (m *MemoryStore) AppendMessage(bookID string, msg domain.Message) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.chats[bookID] = append(m.chats[bookID], msg)
}

// SaveUser registers a user.
func (m *MemoryStore) SaveUser(u domain.User) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[u.ID] = u
	m.email[u.Email] = u.ID
}

// HasUserEmail checks if email exists.
func (m *MemoryStore) HasUserEmail(email string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.email[email]
	return ok
}

// GetUserByEmail looks up a user by email.
func (m *MemoryStore) GetUserByEmail(email string) (domain.User, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if id, ok := m.email[email]; ok {
		u, exists := m.users[id]
		return u, exists
	}
	return domain.User{}, false
}

// GetUserByID returns a user by ID.
func (m *MemoryStore) GetUserByID(id string) (domain.User, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.users[id]
	return u, ok
}

// ListUsers returns all users.
func (m *MemoryStore) ListUsers() []domain.User {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]domain.User, 0, len(m.users))
	for _, u := range m.users {
		res = append(res, u)
	}
	return res
}

// UserCount returns number of users.
func (m *MemoryStore) UserCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.users)
}

// NewSession creates a session token for a user.
func (m *MemoryStore) NewSession(userID string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	token := NewID()
	m.sess[token] = userID
	return token
}

// GetUserByToken returns the user bound to a token.
func (m *MemoryStore) GetUserByToken(token string) (domain.User, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	uid, ok := m.sess[token]
	if !ok {
		return domain.User{}, false
	}
	user, exists := m.users[uid]
	return user, exists
}
