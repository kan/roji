package docker

import (
	"testing"

	"github.com/docker/docker/api/types/events"
)

func TestNewWatcher(t *testing.T) {
	mock := &mockDockerAPI{}
	client := NewClientWithAPI(mock, "network", "localhost")

	watcher := NewWatcher(client)

	if watcher == nil {
		t.Error("NewWatcher() = nil, want non-nil")
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
			client := NewClientWithAPI(mock, "network", "localhost")
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
