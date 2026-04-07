package gqldb

import (
	"context"
	"sync"
	"time"
)

// Session represents an authenticated session with a GQLDB server.
type Session struct {
	ID             uint64
	ServerVersion  string
	Roles          []string
	DefaultGraph   string
	CreatedAt      time.Time
	LastActivity   time.Time
	IsCluster      bool
	ClusterID      string
	PartitionCount int32
	mu             sync.RWMutex
}

// SessionManager manages sessions for the client.
type SessionManager struct {
	session      *Session
	defaultGraph string // standalone default graph for no-auth mode
	mu           sync.RWMutex
}

// NewSessionManager creates a new session manager.
func NewSessionManager() *SessionManager {
	return &SessionManager{}
}

// ClusterInfo contains cluster-related information from login response.
type ClusterInfo struct {
	IsCluster      bool
	ClusterID      string
	PartitionCount int32
}

// Login creates a new session.
func (m *SessionManager) Login(ctx context.Context, sessionID uint64, serverVersion string, roles []string, defaultGraph string, clusterInfo *ClusterInfo) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	session := &Session{
		ID:           sessionID,
		ServerVersion: serverVersion,
		Roles:        roles,
		DefaultGraph: defaultGraph,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
	}

	if clusterInfo != nil {
		session.IsCluster = clusterInfo.IsCluster
		session.ClusterID = clusterInfo.ClusterID
		session.PartitionCount = clusterInfo.PartitionCount
	}

	m.session = session
	return session
}

// Logout clears the current session.
func (m *SessionManager) Logout() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.session = nil
}

// GetSession returns the current session.
func (m *SessionManager) GetSession() *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.session
}

// GetSessionID returns the current session ID.
func (m *SessionManager) GetSessionID() uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.session == nil {
		return 0
	}
	return m.session.ID
}

// IsLoggedIn returns true if there is an active session.
func (m *SessionManager) IsLoggedIn() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.session != nil
}

// UpdateActivity updates the last activity time of the session.
func (m *SessionManager) UpdateActivity() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.session != nil {
		m.session.LastActivity = time.Now()
	}
}

// SetDefaultGraph sets the default graph for the session.
func (m *SessionManager) SetDefaultGraph(graph string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.defaultGraph = graph
	if m.session != nil {
		m.session.DefaultGraph = graph
	}
}

// GetDefaultGraph returns the default graph for the session.
func (m *SessionManager) GetDefaultGraph() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.session != nil && m.session.DefaultGraph != "" {
		return m.session.DefaultGraph
	}
	return m.defaultGraph
}

// HasRole checks if the session has a specific role.
func (s *Session) HasRole(role string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, r := range s.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// IdleDuration returns how long the session has been idle.
func (s *Session) IdleDuration() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return time.Since(s.LastActivity)
}

// Age returns how long the session has been active.
func (s *Session) Age() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return time.Since(s.CreatedAt)
}
