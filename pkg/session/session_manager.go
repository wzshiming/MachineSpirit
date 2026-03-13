package session

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/wzshiming/MachineSpirit/pkg/agent"
	"github.com/wzshiming/MachineSpirit/pkg/memory"
	"github.com/wzshiming/MachineSpirit/pkg/model"
)

var (
	ErrSessionOverloaded = errors.New("session overloaded")
	ErrSessionInactive   = errors.New("session inactive")
	ErrSessionMissingID  = errors.New("session id required")
)

type Manager struct {
	mu         sync.Mutex
	sessions   map[string]*sessionState
	agent      agent.Agent
	mem        memory.Store
	summarizer Summarizer
	maxPending int
	pruneAfter time.Duration
	now        func() time.Time
}

type sessionState struct {
	id         string
	transcript []model.Message
	lastActive time.Time
	pending    int
}

type Option func(m *Manager)

func WithMaxPending(limit int) Option {
	return func(m *Manager) {
		if limit > 0 {
			m.maxPending = limit
		}
	}
}

func WithPruneAfter(d time.Duration) Option {
	return func(m *Manager) {
		if d > 0 {
			m.pruneAfter = d
		}
	}
}

func WithClock(now func() time.Time) Option {
	return func(m *Manager) {
		if now != nil {
			m.now = now
		}
	}
}

// WithMemory attaches a memory store for transcript persistence.
func WithMemory(store memory.Store) Option {
	return func(m *Manager) {
		m.mem = store
	}
}

// WithSummarizer sets a custom summarizer for memory entries.
func WithSummarizer(s Summarizer) Option {
	return func(m *Manager) {
		if s != nil {
			m.summarizer = s
		}
	}
}

func NewManager(agentImpl agent.Agent, opts ...Option) *Manager {
	manager := &Manager{
		sessions:   make(map[string]*sessionState),
		agent:      agentImpl,
		maxPending: 4,
		pruneAfter: 30 * time.Minute,
		now:        time.Now,
		summarizer: SimpleSummarizer{},
	}
	if manager.agent == nil {
		manager.agent = agent.Loop{
			Planner:     agent.EchoPlanner{},
			ToolInvoker: agent.NoopToolInvoker{},
			Composer:    agent.SimpleComposer{},
		}
	}
	for _, opt := range opts {
		opt(manager)
	}
	return manager
}

// HandleEvent processes an inbound event, routing it through the agent loop and returning a response envelope.
func (m *Manager) HandleEvent(ctx context.Context, event model.Event) (model.ResponseEnvelope, error) {
	var envelope model.ResponseEnvelope
	if event.SessionID == "" {
		return envelope, ErrSessionMissingID
	}

	now := m.now()
	if event.Timestamp.IsZero() {
		event.Timestamp = now
	}

	userMessage := model.Message{
		Role:      model.RoleUser,
		Content:   event.Content,
		Timestamp: event.Timestamp,
		Channel:   event.Channel,
		Sender:    event.Sender,
	}

	m.mu.Lock()
	session := m.ensureSession(event.SessionID, now)

	if m.pruneAfter > 0 && now.Sub(session.lastActive) > m.pruneAfter {
		m.mu.Unlock()
		return model.ResponseEnvelope{
			SessionID:  event.SessionID,
			Dropped:    true,
			DropReason: "session inactive",
		}, ErrSessionInactive
	}

	if session.pending >= m.maxPending {
		m.mu.Unlock()
		return model.ResponseEnvelope{
			SessionID:  event.SessionID,
			Dropped:    true,
			DropReason: "queue limit reached",
		}, ErrSessionOverloaded
	}

	session.pending++
	session.lastActive = now
	presence := []model.PresenceUpdate{{Status: model.PresenceTyping, At: now}}
	transcriptCopy := append([]model.Message(nil), session.transcript...)
	m.mu.Unlock()

	reply, err := m.agent.Respond(ctx, agent.Input{
		Event:      event,
		Transcript: transcriptCopy,
	})

	m.mu.Lock()
	defer m.mu.Unlock()

	session.pending--
	session.transcript = append(session.transcript, userMessage)
	envelope.SessionID = event.SessionID
	envelope.Presence = append(envelope.Presence, presence...)

	if err != nil {
		session.lastActive = m.now()
		envelope.Presence = append(envelope.Presence, model.PresenceUpdate{
			Status: model.PresenceActive,
			At:     session.lastActive,
		})
		envelope.Error = err.Error()
		envelope.TranscriptSize = len(session.transcript)
		return envelope, err
	}

	if reply.Role == "" {
		reply.Role = model.RoleAssistant
	}
	if reply.Timestamp.IsZero() {
		reply.Timestamp = m.now()
	}

	session.transcript = append(session.transcript, reply)
	session.lastActive = reply.Timestamp

	if m.mem != nil {
		m.persistMemory(ctx, event.SessionID, userMessage, reply)
	}

	envelope.Messages = []model.Message{reply}
	envelope.Presence = append(envelope.Presence, model.PresenceUpdate{
		Status: model.PresenceActive,
		At:     session.lastActive,
	})
	envelope.TranscriptSize = len(session.transcript)

	return envelope, nil
}

func (m *Manager) PruneInactive() []string {
	if m.pruneAfter <= 0 {
		return nil
	}

	cutoff := m.now().Add(-m.pruneAfter)
	m.mu.Lock()
	defer m.mu.Unlock()

	var removed []string
	for id, session := range m.sessions {
		if session.pending > 0 {
			continue
		}
		if session.lastActive.Before(cutoff) {
			delete(m.sessions, id)
			removed = append(removed, id)
		}
	}
	return removed
}

// StartPruneLoop launches a goroutine that periodically prunes inactive sessions until ctx is cancelled.
func (m *Manager) StartPruneLoop(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = m.pruneAfter
	}
	if interval <= 0 {
		return
	}

	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				m.PruneInactive()
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (m *Manager) Snapshot(sessionID string) (model.SessionSnapshot, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return model.SessionSnapshot{}, false
	}

	transcript := append([]model.Message(nil), session.transcript...)
	return model.SessionSnapshot{
		ID:          sessionID,
		Transcript:  transcript,
		LastActive:  session.lastActive,
		PendingWork: session.pending,
	}, true
}

func (m *Manager) ensureSession(id string, now time.Time) *sessionState {
	session := m.sessions[id]
	if session == nil {
		session = &sessionState{
			id:         id,
			lastActive: now,
		}
		m.sessions[id] = session
	}
	return session
}

const recentMemoryLimit = 50

func (m *Manager) persistMemory(ctx context.Context, sessionID string, user model.Message, reply model.Message) {
	userEntry := formatMemoryLine(sessionID, user)
	replyEntry := formatMemoryLine(sessionID, reply)

	// Append to full conversation log
	full, err := m.mem.Read(ctx, memory.LayerFullConversations)
	if err == nil {
		full = append(full, userEntry, replyEntry)
		_ = m.mem.Write(ctx, memory.LayerFullConversations, full)
	}

	// Maintain capped recent memory
	recent, err := m.mem.Read(ctx, memory.LayerRecent)
	if err == nil {
		recent = append(recent, userEntry, replyEntry)
		if len(recent) > recentMemoryLimit {
			recent = recent[len(recent)-recentMemoryLimit:]
		}
		_ = m.mem.Write(ctx, memory.LayerRecent, recent)
	}

	// Append a discussion summary
	summary := m.summarizer.Summarize(user, reply)
	if strings.TrimSpace(summary) != "" {
		summaries, err := m.mem.Read(ctx, memory.LayerDailySummaries)
		if err == nil {
			summaries = append(summaries, summary)
			_ = m.mem.Write(ctx, memory.LayerDailySummaries, summaries)
		}
	}
}

func formatMemoryLine(sessionID string, msg model.Message) string {
	ts := msg.Timestamp.UTC().Format(time.RFC3339)
	return sessionID + "|" + ts + "|" + string(msg.Role) + "|" + strings.ReplaceAll(msg.Content, "\n", " ")
}
