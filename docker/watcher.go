package docker

import (
	"context"
	"log/slog"

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

// Watch starts watching for container events and returns a channel of events
func (w *Watcher) Watch(ctx context.Context) <-chan ContainerEvent {
	eventCh := make(chan ContainerEvent)

	go func() {
		defer close(eventCh)

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
					slog.Error("docker events error", "error", err)
				}
				return

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
	}()

	return eventCh
}

func (w *Watcher) processEvent(msg events.Message) *ContainerEvent {
	switch msg.Action {
	case "start":
		slog.Debug("container started",
			"container", shortID(msg.ID),
			"name", msg.Actor.Attributes["name"])
		return &ContainerEvent{
			Type:        EventStart,
			ContainerID: msg.ID,
		}

	case "stop", "die":
		slog.Debug("container stopped",
			"container", shortID(msg.ID),
			"name", msg.Actor.Attributes["name"])
		return &ContainerEvent{
			Type:        EventStop,
			ContainerID: msg.ID,
		}
	}

	return nil
}
