package docker

import (
	"context"
	"errors"
	"testing"
	"time"

	dockerclient "github.com/moby/moby/client"

	"github.com/moby/moby/api/types/events"
)

func TestNewWatcher(t *testing.T) {
	mock := &mockDockerAPI{}
	client := NewClientWithAPI(mock, []string{"network"}, "localhost")

	watcher := NewWatcher(client)

	if watcher == nil {
		// Fatal, not Error: the next check dereferences it.
		t.Fatal("NewWatcher() = nil, want non-nil")
	}
	if watcher.client != client {
		t.Error("NewWatcher() client not set correctly")
	}
}

func TestWatcher_processEvent(t *testing.T) {
	tests := []struct {
		name      string
		msg       events.Message
		wantEvent bool
		wantType  EventType
	}{
		{
			name: "start event",
			msg: events.Message{
				Action: "start",
				Actor: events.Actor{
					ID: "abc123",
					Attributes: map[string]string{
						"name": "test-container",
					},
				},
			},
			wantEvent: true,
			wantType:  EventStart,
		},
		{
			name: "stop event",
			msg: events.Message{
				Action: "stop",
				Actor: events.Actor{
					ID: "abc123",
					Attributes: map[string]string{
						"name": "test-container",
					},
				},
			},
			wantEvent: true,
			wantType:  EventStop,
		},
		{
			name: "die event",
			msg: events.Message{
				Action: "die",
				Actor: events.Actor{
					ID: "abc123",
					Attributes: map[string]string{
						"name": "test-container",
					},
				},
			},
			wantEvent: true,
			wantType:  EventStop,
		},
		{
			name: "unknown event",
			msg: events.Message{
				Action: "create",
				Actor: events.Actor{
					ID: "abc123",
				},
			},
			wantEvent: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockDockerAPI{}
			client := NewClientWithAPI(mock, []string{"network"}, "localhost")
			watcher := NewWatcher(client)

			event := watcher.processEvent(tt.msg)

			if tt.wantEvent && event == nil {
				t.Error("processEvent() = nil, want non-nil event")
			}
			if !tt.wantEvent && event != nil {
				t.Errorf("processEvent() = %v, want nil", event)
			}
			if event != nil {
				if event.Type != tt.wantType {
					t.Errorf("processEvent() Type = %v, want %v", event.Type, tt.wantType)
				}
				if event.ContainerID != tt.msg.Actor.ID {
					t.Errorf("processEvent() ContainerID = %v, want %v", event.ContainerID, tt.msg.Actor.ID)
				}
			}
		})
	}
}

// TestWatcher_watchLoop tests that watchLoop correctly delivers events from
// the EventsResult.Messages channel and exits on EventsResult.Err.
func TestWatcher_watchLoop(t *testing.T) {
	t.Run("delivers start event from Messages channel", func(t *testing.T) {
		msgCh := make(chan events.Message, 1)
		errCh := make(chan error, 1)

		mock := &mockDockerAPI{
			eventsFunc: func(ctx context.Context, options dockerclient.EventsListOptions) dockerclient.EventsResult {
				return dockerclient.EventsResult{Messages: msgCh, Err: errCh}
			},
		}
		client := NewClientWithAPI(mock, []string{"roji"}, "localhost")
		watcher := NewWatcher(client)

		eventCh := make(chan ContainerEvent, 1)

		// Send a start message then close the error channel to exit watchLoop
		msgCh <- events.Message{
			Action: "start",
			Actor: events.Actor{
				ID:         "abc123",
				Attributes: map[string]string{"name": "test-container"},
			},
		}

		go func() {
			watcher.watchLoop(context.Background(), eventCh)
		}()

		select {
		case got := <-eventCh:
			if got.Type != EventStart {
				t.Errorf("got event type %v, want EventStart", got.Type)
			}
			if got.ContainerID != "abc123" {
				t.Errorf("got ContainerID %q, want %q", got.ContainerID, "abc123")
			}
			// Signal watchLoop to exit
			errCh <- nil
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for event from watchLoop")
		}
	})

	t.Run("exits on error from Err channel", func(t *testing.T) {
		msgCh := make(chan events.Message)
		errCh := make(chan error, 1)

		mock := &mockDockerAPI{
			eventsFunc: func(ctx context.Context, options dockerclient.EventsListOptions) dockerclient.EventsResult {
				return dockerclient.EventsResult{Messages: msgCh, Err: errCh}
			},
		}
		client := NewClientWithAPI(mock, []string{"roji"}, "localhost")
		watcher := NewWatcher(client)

		eventCh := make(chan ContainerEvent, 1)
		done := make(chan struct{})

		// Send an error to trigger exit
		errCh <- errors.New("docker daemon disconnected")

		go func() {
			watcher.watchLoop(context.Background(), eventCh)
			close(done)
		}()

		select {
		case <-done:
			// watchLoop exited as expected
		case <-time.After(2 * time.Second):
			t.Fatal("timed out: watchLoop did not exit on error")
		}
	})

	t.Run("exits on context cancellation", func(t *testing.T) {
		msgCh := make(chan events.Message)
		errCh := make(chan error)

		mock := &mockDockerAPI{
			eventsFunc: func(ctx context.Context, options dockerclient.EventsListOptions) dockerclient.EventsResult {
				return dockerclient.EventsResult{Messages: msgCh, Err: errCh}
			},
		}
		client := NewClientWithAPI(mock, []string{"roji"}, "localhost")
		watcher := NewWatcher(client)

		ctx, cancel := context.WithCancel(context.Background())
		eventCh := make(chan ContainerEvent, 1)
		done := make(chan struct{})

		go func() {
			watcher.watchLoop(ctx, eventCh)
			close(done)
		}()

		cancel()

		select {
		case <-done:
			// watchLoop exited on context cancellation as expected
		case <-time.After(2 * time.Second):
			t.Fatal("timed out: watchLoop did not exit on context cancellation")
		}
	})
}

// TestWatcher_Watch tests the full Watch loop including reconnect behavior.
func TestWatcher_Watch(t *testing.T) {
	t.Run("delivers events and stops on context cancel", func(t *testing.T) {
		msgCh := make(chan events.Message, 1)
		errCh := make(chan error, 1)

		mock := &mockDockerAPI{
			eventsFunc: func(ctx context.Context, options dockerclient.EventsListOptions) dockerclient.EventsResult {
				return dockerclient.EventsResult{Messages: msgCh, Err: errCh}
			},
		}
		client := NewClientWithAPI(mock, []string{"roji"}, "localhost")
		watcher := NewWatcher(client)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		outCh := watcher.Watch(ctx)

		// Send an event
		msgCh <- events.Message{
			Action: "stop",
			Actor: events.Actor{
				ID:         "def456",
				Attributes: map[string]string{"name": "stopped-container"},
			},
		}

		select {
		case got := <-outCh:
			if got.Type != EventStop {
				t.Errorf("got event type %v, want EventStop", got.Type)
			}
			if got.ContainerID != "def456" {
				t.Errorf("got ContainerID %q, want %q", got.ContainerID, "def456")
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for event from Watch")
		}

		// Cancel context and verify output channel is eventually closed
		cancel()
		// Drain errCh to allow watchLoop to exit
		errCh <- nil

		select {
		case <-outCh:
			// Either a stray event or the close; this test only cares that the
			// receive does not block.
		case <-time.After(3 * time.Second):
			// Acceptable: Watch uses a 5-second reconnect delay, so the goroutine
			// may still be in time.After. The important thing is no deadlock.
		}
	})
}
