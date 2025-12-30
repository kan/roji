package docker

import (
	"context"
	"log/slog"
	"time"

	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
)

// EventType represents the type of container event
type EventType int

const (
	EventStart EventType = iota
	EventStop
)

// ContainerEvent represents a container start/stop event
type ContainerEvent struct {
	Type        EventType
	ContainerID string
}

// Watcher watches for container events on the shared network
type Watcher struct {
	client *Client
}

// NewWatcher creates a new container watcher
func NewWatcher(client *Client) *Watcher {
	return &Watcher{client: client}
}

// Watch starts watching for container events and returns a channel of events.
// Automatically reconnects if the connection is lost.
func (w *Watcher) Watch(ctx context.Context) <-chan ContainerEvent {
	eventCh := make(chan ContainerEvent)

	go func() {
		defer close(eventCh)

		for {
			select {
			case <-ctx.Done():
				return
			default:
				w.watchLoop(ctx, eventCh)

				// Wait before reconnecting (unless context is cancelled)
				select {
				case <-ctx.Done():
					return
				case <-time.After(5 * time.Second):
					slog.Info("reconnecting to docker events...")
				}
			}
		}
	}()

	return eventCh
}

// watchLoop handles a single Events connection
func (w *Watcher) watchLoop(ctx context.Context, eventCh chan<- ContainerEvent) {
	// Filter for container events only
	filterArgs := filters.NewArgs()
	filterArgs.Add("type", "container")
	filterArgs.Add("event", "start")
	filterArgs.Add("event", "stop")
	filterArgs.Add("event", "die")

	msgCh, errCh := w.client.DockerClient().Events(ctx, events.ListOptions{
		Filters: filterArgs,
	})

	for {
		select {
		case <-ctx.Done():
			return

		case err := <-errCh:
			if err != nil {
				slog.Error("docker events error, will reconnect", "error", err)
			}
			return // Exit loop to reconnect

		case msg := <-msgCh:
			event := w.processEvent(msg)
			if event != nil {
				select {
				case eventCh <- *event:
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

func (w *Watcher) processEvent(msg events.Message) *ContainerEvent {
	containerID := msg.Actor.ID

	switch msg.Action {
	case "start":
		slog.Debug("container started",
			"container", shortID(containerID),
			"name", msg.Actor.Attributes["name"])
		return &ContainerEvent{
			Type:        EventStart,
			ContainerID: containerID,
		}

	case "stop", "die":
		slog.Debug("container stopped",
			"container", shortID(containerID),
			"name", msg.Actor.Attributes["name"])
		return &ContainerEvent{
			Type:        EventStop,
			ContainerID: containerID,
		}
	}

	return nil
}
