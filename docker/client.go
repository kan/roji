package docker

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"

	"github.com/kan/roji/config"
)

// shortID returns a shortened container ID for logging (first 12 chars or full ID if shorter)
func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

// DockerAPI defines the interface for Docker API operations
// This interface allows for mocking in tests
type DockerAPI interface {
	ContainerList(ctx context.Context, options container.ListOptions) ([]types.Container, error)
	ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error)
	Events(ctx context.Context, options events.ListOptions) (<-chan events.Message, <-chan error)
	Close() error
}

// Backend represents a proxied service
type Backend struct {
	ContainerID   string
	ContainerName string
	ServiceName   string // docker-compose service name
	ProjectName   string // docker-compose project name
	Host          string // Container IP in the shared network
	Port          int
	Hostname      string // The hostname to route to this backend
	PathPrefix    string // Optional path prefix
}

// Client wraps the Docker client for container discovery
type Client struct {
	docker      DockerAPI
	networkName string // The shared network to watch (e.g., "roji")
	baseDomain  string // Base domain for auto-generated hostnames (e.g., "kan.localhost")
}

// NewClient creates a new Docker client wrapper
func NewClient(networkName, baseDomain string) (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	return NewClientWithAPI(cli, networkName, baseDomain), nil
}

// NewClientWithAPI creates a new client with a custom DockerAPI implementation
// This is useful for testing with mock implementations
func NewClientWithAPI(api DockerAPI, networkName, baseDomain string) *Client {
	return &Client{
		docker:      api,
		networkName: networkName,
		baseDomain:  baseDomain,
	}
}

// Close closes the Docker client
func (c *Client) Close() error {
	return c.docker.Close()
}

// NetworkName returns the network name being watched
func (c *Client) NetworkName() string {
	return c.networkName
}

// BaseDomain returns the base domain for hostnames
func (c *Client) BaseDomain() string {
	return c.baseDomain
}

// buildProjectServiceCounts counts services per project from a list of containers
func buildProjectServiceCounts(containers []types.Container) map[string]int {
	counts := make(map[string]int)
	for _, ctr := range containers {
		// Skip roji itself
		if ctr.Labels["roji.self"] == "true" {
			continue
		}
		if project := ctr.Labels["com.docker.compose.project"]; project != "" {
			counts[project]++
		}
	}
	return counts
}

// DiscoverBackends finds all containers connected to the shared network
func (c *Client) DiscoverBackends(ctx context.Context) ([]*Backend, error) {
	// Filter containers by network
	filterArgs := filters.NewArgs()
	filterArgs.Add("network", c.networkName)

	containers, err := c.docker.ContainerList(ctx, container.ListOptions{
		Filters: filterArgs,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	// Count services per project for hostname generation
	projectServiceCount := buildProjectServiceCounts(containers)

	// Create backends with correct hostnames
	var backends []*Backend
	for _, ctr := range containers {
		backend, err := c.containerToBackend(ctx, ctr, projectServiceCount)
		if err != nil {
			slog.Warn("failed to process container",
				"container", shortID(ctr.ID),
				"error", err)
			continue
		}
		if backend != nil {
			backends = append(backends, backend)
		}
	}

	return backends, nil
}

// GetBackend gets a single backend by container ID
func (c *Client) GetBackend(ctx context.Context, containerID string) (*Backend, error) {
	ctr, err := c.docker.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	// Check if container is on our network
	net, ok := ctr.NetworkSettings.Networks[c.networkName]
	if !ok {
		return nil, nil // Not on our network
	}

	// Count services in the same project for hostname generation
	projectServiceCount := make(map[string]int)
	if project := ctr.Config.Labels["com.docker.compose.project"]; project != "" {
		count, err := c.countProjectServices(ctx, project)
		if err != nil {
			slog.Warn("failed to count project services", "error", err)
			count = 1
		}
		projectServiceCount[project] = count
	}

	return c.inspectToBackend(ctr, net, projectServiceCount)
}

// countProjectServices counts how many services from the same project are on the network
func (c *Client) countProjectServices(ctx context.Context, projectName string) (int, error) {
	filterArgs := filters.NewArgs()
	filterArgs.Add("network", c.networkName)
	filterArgs.Add("label", "com.docker.compose.project="+projectName)

	containers, err := c.docker.ContainerList(ctx, container.ListOptions{
		Filters: filterArgs,
	})
	if err != nil {
		return 0, err
	}

	// Exclude roji itself
	count := 0
	for _, ctr := range containers {
		if ctr.Labels["roji.self"] != "true" {
			count++
		}
	}
	return count, nil
}

func (c *Client) containerToBackend(ctx context.Context, ctr types.Container, projectServiceCount map[string]int) (*Backend, error) {
	// Get the container's IP in our network
	net, ok := ctr.NetworkSettings.Networks[c.networkName]
	if !ok {
		return nil, nil // Not on our network (shouldn't happen with filter)
	}

	// Get full container info for labels
	info, err := c.docker.ContainerInspect(ctx, ctr.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	return c.inspectToBackend(info, net, projectServiceCount)
}

func (c *Client) inspectToBackend(info types.ContainerJSON, net *network.EndpointSettings, projectServiceCount map[string]int) (*Backend, error) {
	// Skip if this is roji itself (avoid self-routing)
	if info.Config.Labels["roji.self"] == "true" {
		return nil, nil
	}

	// Parse labels for configuration
	labelCfg := config.ParseLabels(info.Config.Labels)

	// Determine the port
	port := labelCfg.Port
	if port == 0 {
		port = c.detectPort(info)
	}
	if port == 0 {
		slog.Debug("no port found for container",
			"container", shortID(info.ID),
			"name", info.Name)
		return nil, nil
	}

	// Extract project and service names from compose labels
	projectName := info.Config.Labels["com.docker.compose.project"]
	serviceName := info.Config.Labels["com.docker.compose.service"]
	if serviceName == "" {
		serviceName = strings.TrimPrefix(info.Name, "/")
	}

	// Determine the hostname
	hostname := labelCfg.Host
	if hostname == "" {
		hostname = c.detectHostname(info, projectServiceCount)
	}

	return &Backend{
		ContainerID:   info.ID,
		ContainerName: strings.TrimPrefix(info.Name, "/"),
		ServiceName:   serviceName,
		ProjectName:   projectName,
		Host:          net.IPAddress,
		Port:          port,
		Hostname:      hostname,
		PathPrefix:    labelCfg.PathPrefix,
	}, nil
}

// detectPort finds the first exposed port from the container config
func (c *Client) detectPort(info types.ContainerJSON) int {
	// First, check ExposedPorts from image/container config
	for portSpec := range info.Config.ExposedPorts {
		portStr := strings.Split(string(portSpec), "/")[0]
		if port, err := strconv.Atoi(portStr); err == nil {
			return port
		}
	}

	// Fallback: check published ports (less preferred for internal routing)
	for portSpec := range info.NetworkSettings.Ports {
		portStr := strings.Split(string(portSpec), "/")[0]
		if port, err := strconv.Atoi(portStr); err == nil {
			return port
		}
	}

	return 0
}

// detectHostname generates a hostname based on project/service context
// - Single service in project: project.localhost
// - Multiple services in project: service.project.localhost
// - Non-compose container: container-name.localhost
func (c *Client) detectHostname(info types.ContainerJSON, projectServiceCount map[string]int) string {
	projectName := info.Config.Labels["com.docker.compose.project"]
	serviceName := info.Config.Labels["com.docker.compose.service"]

	// For docker-compose services
	if projectName != "" && serviceName != "" {
		count := projectServiceCount[projectName]
		if count <= 1 {
			// Single service: use project name only
			return config.DefaultHostname(projectName, c.baseDomain)
		}
		// Multiple services: use service.project format
		return config.DefaultHostname(serviceName+"."+projectName, c.baseDomain)
	}

	// Fall back to container name for non-compose containers
	name := strings.TrimPrefix(info.Name, "/")
	return config.DefaultHostname(name, c.baseDomain)
}

// GetProjectBackends gets all backends for a specific project
func (c *Client) GetProjectBackends(ctx context.Context, projectName string) ([]*Backend, error) {
	filterArgs := filters.NewArgs()
	filterArgs.Add("network", c.networkName)
	filterArgs.Add("label", "com.docker.compose.project="+projectName)

	containers, err := c.docker.ContainerList(ctx, container.ListOptions{
		Filters: filterArgs,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	// Count services in this project for hostname generation
	projectServiceCount := buildProjectServiceCounts(containers)

	var backends []*Backend
	for _, ctr := range containers {
		backend, err := c.containerToBackend(ctx, ctr, projectServiceCount)
		if err != nil {
			slog.Warn("failed to process container",
				"container", shortID(ctr.ID),
				"error", err)
			continue
		}
		if backend != nil {
			backends = append(backends, backend)
		}
	}

	return backends, nil
}

// DockerClient returns the underlying Docker API client (for event watching)
func (c *Client) DockerClient() DockerAPI {
	return c.docker
}
