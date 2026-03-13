package agent

import (
	"sync"
	"time"
)

// Fact represents a piece of information stored in memory.
type Fact struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	Timestamp time.Time `json:"timestamp"`
}

// Memory provides storage and retrieval of facts for agent decision-making.
type Memory interface {
	// Store saves a fact to memory.
	Store(key, value string)
	// Retrieve fetches a fact by key. Returns empty string if not found.
	Retrieve(key string) string
	// Search returns all facts whose keys or values contain the query string.
	Search(query string) []Fact
	// All returns all facts in memory.
	All() []Fact
	// Clear removes all facts from memory.
	Clear()
}

// InMemoryStore is a simple in-memory implementation of Memory.
type InMemoryStore struct {
	mu    sync.RWMutex
	facts map[string]Fact
}

// NewInMemoryStore creates a new in-memory fact store.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		facts: make(map[string]Fact),
	}
}

// Store saves a fact to memory.
func (m *InMemoryStore) Store(key, value string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.facts[key] = Fact{
		Key:       key,
		Value:     value,
		Timestamp: time.Now(),
	}
}

// Retrieve fetches a fact by key.
func (m *InMemoryStore) Retrieve(key string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if fact, ok := m.facts[key]; ok {
		return fact.Value
	}
	return ""
}

// Search returns all facts matching the query.
func (m *InMemoryStore) Search(query string) []Fact {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var results []Fact
	for _, fact := range m.facts {
		if contains(fact.Key, query) || contains(fact.Value, query) {
			results = append(results, fact)
		}
	}
	return results
}

// All returns all facts in memory.
func (m *InMemoryStore) All() []Fact {
	m.mu.RLock()
	defer m.mu.RUnlock()
	results := make([]Fact, 0, len(m.facts))
	for _, fact := range m.facts {
		results = append(results, fact)
	}
	return results
}

// Clear removes all facts from memory.
func (m *InMemoryStore) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.facts = make(map[string]Fact)
}

// contains checks if the haystack contains the needle (case-sensitive).
func contains(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) &&
		findSubstring(haystack, needle)
}

func findSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
