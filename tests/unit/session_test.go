package unit

import (
	"context"
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

func TestSessionManager(t *testing.T) {
	sm := gqldb.NewSessionManager()

	// Initially not logged in
	if sm.IsLoggedIn() {
		t.Error("expected not logged in initially")
	}

	if sm.GetSessionID() != 0 {
		t.Error("expected session ID 0 when not logged in")
	}

	// Login
	session := sm.Login(context.Background(), 12345, "1.0.0", []string{"admin", "user"}, "testGraph", nil)

	if !sm.IsLoggedIn() {
		t.Error("expected logged in after Login")
	}

	if sm.GetSessionID() != 12345 {
		t.Errorf("expected session ID 12345, got %d", sm.GetSessionID())
	}

	if session.ServerVersion != "1.0.0" {
		t.Errorf("expected server version 1.0.0, got %s", session.ServerVersion)
	}

	if len(session.Roles) != 2 {
		t.Errorf("expected 2 roles, got %d", len(session.Roles))
	}

	if sm.GetDefaultGraph() != "testGraph" {
		t.Errorf("expected default graph testGraph, got %s", sm.GetDefaultGraph())
	}

	// Update activity
	oldActivity := session.LastActivity
	time.Sleep(10 * time.Millisecond)
	sm.UpdateActivity()
	if session.LastActivity == oldActivity {
		t.Error("expected last activity to be updated")
	}

	// Set default graph
	sm.SetDefaultGraph("newGraph")
	if sm.GetDefaultGraph() != "newGraph" {
		t.Errorf("expected default graph newGraph, got %s", sm.GetDefaultGraph())
	}

	// Logout
	sm.Logout()

	if sm.IsLoggedIn() {
		t.Error("expected not logged in after Logout")
	}

	if sm.GetSession() != nil {
		t.Error("expected nil session after Logout")
	}
}

func TestSessionHasRole(t *testing.T) {
	sm := gqldb.NewSessionManager()
	session := sm.Login(context.Background(), 1, "1.0.0", []string{"admin", "user"}, "", nil)

	if !session.HasRole("admin") {
		t.Error("expected HasRole(admin) to be true")
	}

	if !session.HasRole("user") {
		t.Error("expected HasRole(user) to be true")
	}

	if session.HasRole("superuser") {
		t.Error("expected HasRole(superuser) to be false")
	}
}

func TestSessionDurations(t *testing.T) {
	sm := gqldb.NewSessionManager()
	session := sm.Login(context.Background(), 1, "1.0.0", nil, "", nil)

	time.Sleep(50 * time.Millisecond)

	age := session.Age()
	if age < 50*time.Millisecond {
		t.Errorf("expected age >= 50ms, got %v", age)
	}

	idle := session.IdleDuration()
	if idle < 50*time.Millisecond {
		t.Errorf("expected idle >= 50ms, got %v", idle)
	}

	// Update activity and check idle resets
	sm.UpdateActivity()
	idle = session.IdleDuration()
	if idle > 10*time.Millisecond {
		t.Errorf("expected idle < 10ms after update, got %v", idle)
	}
}
