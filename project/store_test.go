package project

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStore_InMemory(t *testing.T) {
	store := NewStore("")

	// Test Update and Get
	p := &Project{
		Name:       "test-project",
		WorkingDir: "/home/user/test",
		Services:   []string{"web", "api"},
		Active:     true,
		LastActive: time.Now(),
	}

	store.Update(p)

	got := store.Get("test-project")
	if got == nil {
		t.Fatal("expected project to be stored")
	}
	if got.Name != "test-project" {
		t.Errorf("Name = %q, want %q", got.Name, "test-project")
	}
	if got.WorkingDir != "/home/user/test" {
		t.Errorf("WorkingDir = %q, want %q", got.WorkingDir, "/home/user/test")
	}
	if len(got.Services) != 2 {
		t.Errorf("Services count = %d, want 2", len(got.Services))
	}
}

func TestStore_ListActive(t *testing.T) {
	store := NewStore("")

	store.Update(&Project{Name: "active1", Active: true})
	store.Update(&Project{Name: "active2", Active: true})
	store.Update(&Project{Name: "inactive1", Active: false})

	active := store.ListActive()
	if len(active) != 2 {
		t.Errorf("active count = %d, want 2", len(active))
	}

	inactive := store.ListInactive()
	if len(inactive) != 1 {
		t.Errorf("inactive count = %d, want 1", len(inactive))
	}
}

func TestStore_SetActive(t *testing.T) {
	store := NewStore("")

	store.Update(&Project{Name: "test", Active: true})

	store.SetActive("test", false)
	got := store.Get("test")
	if got.Active {
		t.Error("project should be inactive")
	}

	store.SetActive("test", true)
	got = store.Get("test")
	if !got.Active {
		t.Error("project should be active")
	}
}

func TestStore_SetAllInactive(t *testing.T) {
	store := NewStore("")

	store.Update(&Project{Name: "p1", Active: true})
	store.Update(&Project{Name: "p2", Active: true})

	store.SetAllInactive()

	active := store.ListActive()
	if len(active) != 0 {
		t.Errorf("all projects should be inactive, got %d active", len(active))
	}
}

func TestStore_Remove(t *testing.T) {
	store := NewStore("")

	store.Update(&Project{Name: "test", Active: true})
	store.Remove("test")

	got := store.Get("test")
	if got != nil {
		t.Error("project should be removed")
	}
}

func TestStore_Count(t *testing.T) {
	store := NewStore("")

	if store.Count() != 0 {
		t.Errorf("initial count = %d, want 0", store.Count())
	}

	store.Update(&Project{Name: "p1"})
	store.Update(&Project{Name: "p2"})

	if store.Count() != 2 {
		t.Errorf("count = %d, want 2", store.Count())
	}
}

func TestStore_Persistence(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "roji-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "projects.json")

	// Create store and add project
	store1 := NewStore(filePath)
	store1.Update(&Project{
		Name:       "persistent-project",
		WorkingDir: "/home/user/project",
		Services:   []string{"web"},
		Active:     true,
		LastActive: time.Now(),
	})

	// Wait for async save
	time.Sleep(100 * time.Millisecond)

	// Create new store from same file
	store2 := NewStore(filePath)

	got := store2.Get("persistent-project")
	if got == nil {
		t.Fatal("project should be loaded from file")
	}
	if got.Name != "persistent-project" {
		t.Errorf("Name = %q, want %q", got.Name, "persistent-project")
	}
	if got.WorkingDir != "/home/user/project" {
		t.Errorf("WorkingDir = %q, want %q", got.WorkingDir, "/home/user/project")
	}
}

func TestStore_GetReturnsCopy(t *testing.T) {
	store := NewStore("")

	store.Update(&Project{Name: "test", WorkingDir: "/original"})

	got := store.Get("test")
	got.WorkingDir = "/modified"

	original := store.Get("test")
	if original.WorkingDir != "/original" {
		t.Error("Get should return a copy, not the original")
	}
}
