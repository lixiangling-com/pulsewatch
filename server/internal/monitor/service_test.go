package monitor

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCreateNormalizesAndValidatesInput(t *testing.T) {
	store := newMemoryStore()
	service := NewService(store)
	fixed := time.Date(2026, 9, 18, 8, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return fixed }
	userID := uuid.New()

	created, err := service.Create(context.Background(), userID, CreateInput{
		Name: "  API  ", URL: " https://example.com/health?token=secret ", IntervalMinutes: 5, ExpectedStatus: 204,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Name != "API" || created.URL != "https://example.com/health?token=secret" || created.Status != StatusPending || created.ConfigVersion != 1 || !created.NextCheckAt.Equal(fixed) {
		t.Fatalf("created monitor = %#v", created)
	}

	tests := []struct {
		name  string
		input CreateInput
		field string
	}{
		{"blank name", CreateInput{Name: "  ", URL: "https://example.com", IntervalMinutes: 5, ExpectedStatus: 200}, "name"},
		{"long name", CreateInput{Name: strings.Repeat("名", 81), URL: "https://example.com", IntervalMinutes: 5, ExpectedStatus: 200}, "name"},
		{"scheme", CreateInput{Name: "x", URL: "ftp://example.com", IntervalMinutes: 5, ExpectedStatus: 200}, "url"},
		{"credentials", CreateInput{Name: "x", URL: "https://user:pass@example.com", IntervalMinutes: 5, ExpectedStatus: 200}, "url"},
		{"fragment", CreateInput{Name: "x", URL: "https://example.com/#secret", IntervalMinutes: 5, ExpectedStatus: 200}, "url"},
		{"localhost", CreateInput{Name: "x", URL: "http://localhost:8080", IntervalMinutes: 5, ExpectedStatus: 200}, "url"},
		{"local domain", CreateInput{Name: "x", URL: "http://service.local", IntervalMinutes: 5, ExpectedStatus: 200}, "url"},
		{"private ipv4", CreateInput{Name: "x", URL: "http://10.0.0.1", IntervalMinutes: 5, ExpectedStatus: 200}, "url"},
		{"loopback ipv6", CreateInput{Name: "x", URL: "http://[::1]", IntervalMinutes: 5, ExpectedStatus: 200}, "url"},
		{"metadata", CreateInput{Name: "x", URL: "http://169.254.169.254", IntervalMinutes: 5, ExpectedStatus: 200}, "url"},
		{"interval", CreateInput{Name: "x", URL: "https://example.com", IntervalMinutes: 2, ExpectedStatus: 200}, "interval_minutes"},
		{"status", CreateInput{Name: "x", URL: "https://example.com", IntervalMinutes: 5, ExpectedStatus: 99}, "expected_status"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := service.Create(context.Background(), userID, test.input)
			var invalid *ValidationError
			if !errors.As(err, &invalid) || len(invalid.Fields[test.field]) == 0 {
				t.Fatalf("error = %#v, want validation field %q", err, test.field)
			}
		})
	}
}

func TestUpdateVersionAndActionSemantics(t *testing.T) {
	store := newMemoryStore()
	service := NewService(store)
	clock := time.Date(2026, 9, 18, 8, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return clock }
	userID := uuid.New()
	created, err := service.Create(context.Background(), userID, validCreate("API"))
	if err != nil {
		t.Fatal(err)
	}

	name := "Renamed"
	renamed, err := service.Update(context.Background(), userID, created.ID, UpdateInput{Name: &name})
	if err != nil || renamed.ConfigVersion != 1 || renamed.Status != StatusPending {
		t.Fatalf("rename = %#v, %v", renamed, err)
	}

	clock = clock.Add(time.Minute)
	interval := 10
	changed, err := service.Update(context.Background(), userID, created.ID, UpdateInput{IntervalMinutes: &interval})
	if err != nil || changed.ConfigVersion != 2 || changed.Status != StatusPending || !changed.NextCheckAt.Equal(clock) {
		t.Fatalf("configuration update = %#v, %v", changed, err)
	}

	paused, err := service.Pause(context.Background(), userID, created.ID)
	if err != nil || paused.Status != StatusPaused || paused.ConfigVersion != 3 {
		t.Fatalf("pause = %#v, %v", paused, err)
	}
	repeatedPause, _ := service.Pause(context.Background(), userID, created.ID)
	if repeatedPause.ConfigVersion != 3 {
		t.Fatalf("repeated pause version = %d", repeatedPause.ConfigVersion)
	}

	status := 201
	whilePaused, err := service.Update(context.Background(), userID, created.ID, UpdateInput{ExpectedStatus: &status})
	if err != nil || whilePaused.Status != StatusPaused || whilePaused.ConfigVersion != 4 {
		t.Fatalf("paused update = %#v, %v", whilePaused, err)
	}

	clock = clock.Add(time.Minute)
	resumed, err := service.Resume(context.Background(), userID, created.ID)
	if err != nil || resumed.Status != StatusPending || resumed.ConfigVersion != 5 || !resumed.NextCheckAt.Equal(clock) {
		t.Fatalf("resume = %#v, %v", resumed, err)
	}
	repeatedResume, _ := service.Resume(context.Background(), userID, created.ID)
	if repeatedResume.ConfigVersion != 5 {
		t.Fatalf("repeated resume version = %d", repeatedResume.ConfigVersion)
	}

	if _, err := service.Update(context.Background(), userID, created.ID, UpdateInput{}); err == nil {
		t.Fatal("empty patch succeeded")
	}
}

func validCreate(name string) CreateInput {
	return CreateInput{Name: name, URL: "https://example.com/health", IntervalMinutes: 5, ExpectedStatus: 200}
}

type memoryStore struct {
	mu       sync.Mutex
	monitors map[uuid.UUID]Monitor
}

func newMemoryStore() *memoryStore {
	return &memoryStore{monitors: map[uuid.UUID]Monitor{}}
}

func (s *memoryStore) Create(_ context.Context, userID uuid.UUID, value Monitor) (Monitor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for _, monitor := range s.monitors {
		if monitor.UserID == userID {
			count++
		}
	}
	if count >= MaxPerUser {
		return Monitor{}, ErrLimit
	}
	value.CreatedAt = value.NextCheckAt
	value.UpdatedAt = value.NextCheckAt
	s.monitors[value.ID] = value
	return value, nil
}

func (s *memoryStore) List(_ context.Context, userID uuid.UUID, page, pageSize int) ([]Monitor, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := []Monitor{}
	for _, item := range s.monitors {
		if item.UserID == userID {
			items = append(items, item)
		}
	}
	start := (page - 1) * pageSize
	if start >= len(items) {
		return []Monitor{}, int64(len(items)), nil
	}
	end := min(start+pageSize, len(items))
	return items[start:end], int64(len(items)), nil
}

func (s *memoryStore) Get(_ context.Context, userID, id uuid.UUID) (Monitor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.monitors[id]
	if !ok || item.UserID != userID {
		return Monitor{}, ErrNotFound
	}
	return item, nil
}

func (s *memoryStore) Mutate(_ context.Context, userID, id uuid.UUID, mutate MutateFunc) (Monitor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.monitors[id]
	if !ok || item.UserID != userID {
		return Monitor{}, ErrNotFound
	}
	next, err := mutate(item)
	if err != nil {
		return Monitor{}, err
	}
	s.monitors[id] = next
	return next, nil
}

func (s *memoryStore) Delete(_ context.Context, userID, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.monitors[id]
	if !ok || item.UserID != userID {
		return ErrNotFound
	}
	delete(s.monitors, id)
	return nil
}
