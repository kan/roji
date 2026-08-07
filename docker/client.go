// Package docker provides Docker API client functionality for service discovery.
//
// It handles:
//   - Container discovery on the roji network
//   - Docker Compose project detection
//   - Backend extraction from container metadata
//   - Event monitoring for dynamic updates
package docker

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	dockerclient "github.com/moby/moby/client"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"

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
	ContainerList(ctx context.Context, options dockerclient.ContainerListOptions) (dockerclient.ContainerListResult, error)
	ContainerInspect(ctx context.Context, containerID string, options dockerclient.ContainerInspectOptions) (dockerclient.ContainerInspectResult, error)
	ContainerRestart(ctx context.Context, containerID string, options dockerclient.ContainerRestartOptions) (dockerclient.ContainerRestartResult, error)
	Events(ctx context.Context, options dockerclient.EventsListOptions) dockerclient.EventsResult
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
	Hostname      string              // The hostname to route to this backend
	PathPrefix    string              // Optional path prefix; see config.RouteConfig.PathPrefix
	Warning       string              // Warning message (e.g., "no port exposed")
	Network       string              // Docker network name this container was found on
	MockRoutes    []*config.MockRoute // Mock responses defined via labels
	BasicAuth     *config.BasicAuth   // Basic authentication (optional)
}

// ProjectInfo contains docker-compose project metadata
type ProjectInfo struct {
	Name        string   // Project name from com.docker.compose.project
	WorkingDir  string   // Working directory from com.docker.compose.project.working_dir
	ConfigFiles string   // Config files from com.docker.compose.project.config_files
	Services    []string // List of service names in this project
}

// Client wraps the Docker client for container discovery
type Client struct {
	docker     DockerAPI
	networks   []string // The shared networks to watch (e.g., ["roji", "custom"])
	baseDomain string   // Base domain for auto-generated hostnames (e.g., "kan.localhost")
}

// NewClient creates a new Docker client wrapper
func NewClient(networks []string, baseDomain string) (*Client, error) {
	cli, err := dockerclient.New(dockerclient.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	return NewClientWithAPI(cli, networks, baseDomain), nil
}

// NewClientWithAPI creates a new client with a custom DockerAPI implementation
// This is useful for testing with mock implementations
func NewClientWithAPI(api DockerAPI, networks []string, baseDomain string) *Client {
	return &Client{
		docker:     api,
		networks:   networks,
		baseDomain: baseDomain,
	}
}

// Close closes the Docker client
func (c *Client) Close() error {
	return c.docker.Close()
}

// Networks returns the network names being watched
func (c *Client) Networks() []string {
	return c.networks
}

// BaseDomain returns the base domain for hostnames
func (c *Client) BaseDomain() string {
	return c.baseDomain
}

// buildProjectServiceCounts counts services per project from a list of containers
func buildProjectServiceCounts(containers []container.Summary) map[string]int {
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

// DiscoverBackends finds all containers connected to any of the watched networks
func (c *Client) DiscoverBackends(ctx context.Context) ([]*Backend, error) {
	// Add timeout for Docker API call
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Collect containers from all networks (deduplicated by container ID)
	seenContainers := make(map[string]container.Summary)
	containerNetworks := make(map[string]string) // containerID -> networkName

	for _, networkName := range c.networks {
		filterArgs := make(dockerclient.Filters).Add("network", networkName)

		result, err := c.docker.ContainerList(ctx, dockerclient.ContainerListOptions{
			Filters: filterArgs,
		})
		if err != nil {
			slog.Warn("failed to list containers for network", "network", networkName, "error", err)
			continue
		}

		for _, ctr := range result.Items {
			if _, seen := seenContainers[ctr.ID]; !seen {
				seenContainers[ctr.ID] = ctr
				containerNetworks[ctr.ID] = networkName
			}
		}
	}

	// Convert map to slice
	var allContainers []container.Summary
	for _, ctr := range seenContainers {
		allContainers = append(allContainers, ctr)
	}

	// Count services per project for hostname generation
	projectServiceCount := buildProjectServiceCounts(allContainers)

	// Create backends with correct hostnames
	var backends []*Backend
	for _, ctr := range allContainers {
		networkName := containerNetworks[ctr.ID]
		backend, err := c.containerToBackend(ctx, ctr, networkName, projectServiceCount)
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
	// Add timeout for Docker API call
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	result, err := c.docker.ContainerInspect(ctx, containerID, dockerclient.ContainerInspectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}
	ctr := result.Container

	// Check if container is on any of our networks
	var foundNetwork string
	var net *network.EndpointSettings
	for _, networkName := range c.networks {
		if n, ok := ctr.NetworkSettings.Networks[networkName]; ok {
			foundNetwork = networkName
			net = n
			break
		}
	}
	if net == nil {
		return nil, nil // Not on any of our networks
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

	return c.inspectToBackend(ctr, foundNetwork, net, projectServiceCount)
}

// countProjectServices counts how many services from the same project are on any of our networks
func (c *Client) countProjectServices(ctx context.Context, projectName string) (int, error) {
	// Add timeout for Docker API call
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Count unique containers across all networks
	seenIDs := make(map[string]bool)

	for _, networkName := range c.networks {
		filterArgs := make(dockerclient.Filters).
			Add("network", networkName).
			Add("label", "com.docker.compose.project="+projectName)

		result, err := c.docker.ContainerList(ctx, dockerclient.ContainerListOptions{
			Filters: filterArgs,
		})
		if err != nil {
			continue
		}

		for _, ctr := range result.Items {
			if ctr.Labels["roji.self"] != "true" {
				seenIDs[ctr.ID] = true
			}
		}
	}

	return len(seenIDs), nil
}

func (c *Client) containerToBackend(ctx context.Context, ctr container.Summary, networkName string, projectServiceCount map[string]int) (*Backend, error) {
	// Get the container's IP in the specified network
	net, ok := ctr.NetworkSettings.Networks[networkName]
	if !ok {
		return nil, nil // Not on the specified network (shouldn't happen with filter)
	}

	// Get full container info for labels
	result, err := c.docker.ContainerInspect(ctx, ctr.ID, dockerclient.ContainerInspectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	return c.inspectToBackend(result.Container, networkName, net, projectServiceCount)
}

func (c *Client) inspectToBackend(info container.InspectResponse, networkName string, net *network.EndpointSettings, projectServiceCount map[string]int) (*Backend, error) {
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

	// Track warning for containers without exposed ports
	var warning string
	if port == 0 {
		warning = "no port exposed"
		slog.Debug("no port found for container",
			"container", shortID(info.ID),
			"name", info.Name)
	}

	// Extract project and service names from compose labels
	projectName := info.Config.Labels["com.docker.compose.project"]
	serviceName := info.Config.Labels["com.docker.compose.service"]
	if serviceName == "" {
		serviceName = trimLeadingSlash(info.Name)
	}

	// Determine the hostname
	hostname := labelCfg.Host
	if hostname == "" {
		hostname = c.detectHostname(info, projectServiceCount)
	}

	return &Backend{
		ContainerID:   info.ID,
		ContainerName: trimLeadingSlash(info.Name),
		ServiceName:   serviceName,
		ProjectName:   projectName,
		Host:          net.IPAddress.String(),
		Port:          port,
		Hostname:      hostname,
		PathPrefix:    labelCfg.PathPrefix,
		Warning:       warning,
		Network:       networkName,
		MockRoutes:    labelCfg.MockRoutes,
		BasicAuth:     labelCfg.BasicAuth,
	}, nil
}

func trimLeadingSlash(name string) string {
	if len(name) > 0 && name[0] == '/' {
		return name[1:]
	}
	return name
}

// detectPort finds the first exposed port from the container config
func (c *Client) detectPort(info container.InspectResponse) int {
	// First, check ExposedPorts from image/container config
	for portSpec := range info.Config.ExposedPorts {
		return int(portSpec.Num())
	}

	// Fallback: check published ports (less preferred for internal routing)
	for portSpec := range info.NetworkSettings.Ports {
		return int(portSpec.Num())
	}

	return 0
}

// detectHostname generates a hostname based on project/service context
// - Single service in project: project.localhost
// - Multiple services in project: service-project.localhost (hyphen to keep single subdomain level)
// - Non-compose container: container-name.localhost
func (c *Client) detectHostname(info container.InspectResponse, projectServiceCount map[string]int) string {
	projectName := info.Config.Labels["com.docker.compose.project"]
	serviceName := info.Config.Labels["com.docker.compose.service"]

	// For docker-compose services
	if projectName != "" && serviceName != "" {
		count := projectServiceCount[projectName]
		if count <= 1 {
			// Single service: use project name only
			return config.DefaultHostname(projectName, c.baseDomain)
		}
		// Multiple services: use service-project format (hyphen keeps wildcard cert valid)
		return config.DefaultHostname(serviceName+"-"+projectName, c.baseDomain)
	}

	// Fall back to container name for non-compose containers
	name := trimLeadingSlash(info.Name)
	return config.DefaultHostname(name, c.baseDomain)
}

// GetProjectBackends gets all backends for a specific project across all watched networks
func (c *Client) GetProjectBackends(ctx context.Context, projectName string) ([]*Backend, error) {
	// Add timeout for Docker API call
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Collect containers from all networks (deduplicated by container ID)
	seenContainers := make(map[string]container.Summary)
	containerNetworks := make(map[string]string)

	for _, networkName := range c.networks {
		filterArgs := make(dockerclient.Filters).
			Add("network", networkName).
			Add("label", "com.docker.compose.project="+projectName)

		result, err := c.docker.ContainerList(ctx, dockerclient.ContainerListOptions{
			Filters: filterArgs,
		})
		if err != nil {
			continue
		}

		for _, ctr := range result.Items {
			if _, seen := seenContainers[ctr.ID]; !seen {
				seenContainers[ctr.ID] = ctr
				containerNetworks[ctr.ID] = networkName
			}
		}
	}

	// Convert to slice
	var allContainers []container.Summary
	for _, ctr := range seenContainers {
		allContainers = append(allContainers, ctr)
	}

	// Count services in this project for hostname generation
	projectServiceCount := buildProjectServiceCounts(allContainers)

	var backends []*Backend
	for _, ctr := range allContainers {
		networkName := containerNetworks[ctr.ID]
		backend, err := c.containerToBackend(ctx, ctr, networkName, projectServiceCount)
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

// GetProjectInfo extracts project metadata from a container
func (c *Client) GetProjectInfo(ctx context.Context, containerID string) (*ProjectInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	result, err := c.docker.ContainerInspect(ctx, containerID, dockerclient.ContainerInspectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}
	info := result.Container

	projectName := info.Config.Labels["com.docker.compose.project"]
	if projectName == "" {
		return nil, nil // Not a docker-compose container
	}

	return &ProjectInfo{
		Name:        projectName,
		WorkingDir:  info.Config.Labels["com.docker.compose.project.working_dir"],
		ConfigFiles: info.Config.Labels["com.docker.compose.project.config_files"],
	}, nil
}

// DiscoverProjects finds all docker-compose projects on any of the watched networks
func (c *Client) DiscoverProjects(ctx context.Context) (map[string]*ProjectInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Collect containers from all networks (deduplicated)
	seenContainers := make(map[string]container.Summary)

	for _, networkName := range c.networks {
		filterArgs := make(dockerclient.Filters).Add("network", networkName)

		result, err := c.docker.ContainerList(ctx, dockerclient.ContainerListOptions{
			Filters: filterArgs,
		})
		if err != nil {
			continue
		}

		for _, ctr := range result.Items {
			if _, seen := seenContainers[ctr.ID]; !seen {
				seenContainers[ctr.ID] = ctr
			}
		}
	}

	projects := make(map[string]*ProjectInfo)

	for _, ctr := range seenContainers {
		// Skip roji itself
		if ctr.Labels["roji.self"] == "true" {
			continue
		}

		projectName := ctr.Labels["com.docker.compose.project"]
		if projectName == "" {
			continue
		}

		serviceName := ctr.Labels["com.docker.compose.service"]

		if existing, ok := projects[projectName]; ok {
			// Add service to existing project
			existing.Services = append(existing.Services, serviceName)
		} else {
			// Create new project entry
			projects[projectName] = &ProjectInfo{
				Name:        projectName,
				WorkingDir:  ctr.Labels["com.docker.compose.project.working_dir"],
				ConfigFiles: ctr.Labels["com.docker.compose.project.config_files"],
				Services:    []string{serviceName},
			}
		}
	}

	return projects, nil
}

// RestartContainer restarts a container by ID
func (c *Client) RestartContainer(ctx context.Context, containerID string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	_, err := c.docker.ContainerRestart(ctx, containerID, dockerclient.ContainerRestartOptions{})
	return err
}
