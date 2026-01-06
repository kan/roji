package project

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Project represents a docker-compose project
type Project struct {
	Name        string    `json:"name"`
	WorkingDir  string    `json:"working_dir"`
	ConfigFiles string    `json:"config_files"`
	LastActive  time.Time `json:"last_active"`
	Services    []string  `json:"services"`
	Active      bool      `json:"active"`
}

// Store manages project history with persistence
type Store struct {
	mu       sync.RWMutex
	projects map[string]*Project
	filePath string
}

// NewStore creates a new project store with optional persistence
func NewStore(filePath string) *Store {
	s := &Store{
		projects: make(map[string]*Project),
		filePath: filePath,
	}

	// Load existing data if file exists
	if filePath != "" {
		if err := s.load(); err != nil {
			slog.Debug("no existing project history", "path", filePath, "error", err)
		}
	}

	return s
}

// Update adds or updates a project in the store
func (s *Store) Update(p *Project) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.projects[p.Name]
	if ok {
		// Update existing project
		existing.WorkingDir = p.WorkingDir
		existing.ConfigFiles = p.ConfigFiles
		existing.LastActive = p.LastActive
		existing.Services = p.Services
		existing.Active = p.Active
	} else {
		// Add new project
		s.projects[p.Name] = p
	}

	// Persist asynchronously
	go s.save()
}

// SetActive marks a project as active or inactive
func (s *Store) SetActive(name string, active bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if p, ok := s.projects[name]; ok {
		p.Active = active
		if active {
			p.LastActive = time.Now()
		}
		go s.save()
	}
}

// SetAllInactive marks all projects as inactive
func (s *Store) SetAllInactive() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, p := range s.projects {
		p.Active = false
	}
	go s.save()
}

// Get returns a project by name
func (s *Store) Get(name string) *Project {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if p, ok := s.projects[name]; ok {
		// Return a copy
		copy := *p
		return &copy
	}
	return nil
}

// List returns all projects, optionally filtered by active status
func (s *Store) List(activeOnly bool) []*Project {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Project
	for _, p := range s.projects {
		if activeOnly && !p.Active {
			continue
		}
		// Return copies
		copy := *p
		result = append(result, &copy)
	}
	return result
}

// ListActive returns only active projects
func (s *Store) ListActive() []*Project {
	return s.List(true)
}

// ListInactive returns only inactive projects
func (s *Store) ListInactive() []*Project {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Project
	for _, p := range s.projects {
		if !p.Active {
			copy := *p
			result = append(result, &copy)
		}
	}
	return result
}

// Remove removes a project from the store
func (s *Store) Remove(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.projects, name)
	go s.save()
}

// save persists the store to disk
func (s *Store) save() {
	if s.filePath == "" {
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Ensure directory exists
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Error("failed to create directory for project store", "error", err)
		return
	}

	data, err := json.MarshalIndent(s.projects, "", "  ")
	if err != nil {
		slog.Error("failed to marshal project store", "error", err)
		return
	}

	if err := os.WriteFile(s.filePath, data, 0644); err != nil {
		slog.Error("failed to save project store", "error", err)
		return
	}

	slog.Debug("project store saved", "path", s.filePath, "count", len(s.projects))
}

// load reads the store from disk
func (s *Store) load() error {
	if s.filePath == "" {
		return nil
	}

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &s.projects)
}

// Count returns the number of projects in the store
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.projects)
}
