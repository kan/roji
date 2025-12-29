package docker

import (
	"context"
	"fmt"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
)

// mockDockerAPI is a mock implementation of DockerAPI for testing
type mockDockerAPI struct {
	containers     []types.Container
	inspectMap     map[string]types.ContainerJSON
	containerList  func(ctx context.Context, options container.ListOptions) ([]types.Container, error)
	containerInspect func(ctx context.Context, containerID string) (types.ContainerJSON, error)
	events         func(ctx context.Context, options events.ListOptions) (<-chan events.Message, <-chan error)
}

func (m *mockDockerAPI) ContainerList(ctx context.Context, options container.ListOptions) ([]types.Container, error) {
	if m.containerList != nil {
		return m.containerList(ctx, options)
	}
	return m.containers, nil
}

func (m *mockDockerAPI) ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error) {
	if m.containerInspect != nil {
		return m.containerInspect(ctx, containerID)
	}
	if info, ok := m.inspectMap[containerID]; ok {
		return info, nil
	}
	return types.ContainerJSON{}, nil
}

func (m *mockDockerAPI) Events(ctx context.Context, options events.ListOptions) (<-chan events.Message, <-chan error) {
	if m.events != nil {
		return m.events(ctx, options)
	}
	msgCh := make(chan events.Message)
	errCh := make(chan error)
	close(msgCh)
	close(errCh)
	return msgCh, errCh
}

func (m *mockDockerAPI) Close() error {
	return nil
}

// Test helper to create a mock container
func createMockContainer(id, name, serviceName, projectName string, port int, networkName string) types.Container {
	labels := map[string]string{}
	if serviceName != "" {
		labels["com.docker.compose.service"] = serviceName
	}
	if projectName != "" {
		labels["com.docker.compose.project"] = projectName
	}

	return types.Container{
		ID:     id,
		Names:  []string{"/" + name},
		Labels: labels,
		NetworkSettings: &types.SummaryNetworkSettings{
			Networks: map[string]*network.EndpointSettings{
				networkName: {
					IPAddress: "172.18.0.2",
				},
			},
		},
	}
}

// Test helper to create a mock ContainerJSON
func createMockContainerJSON(id, name, serviceName, projectName string, port int, networkName string) types.ContainerJSON {
	labels := map[string]string{}
	if serviceName != "" {
		labels["com.docker.compose.service"] = serviceName
	}
	if projectName != "" {
		labels["com.docker.compose.project"] = projectName
	}

	exposedPorts := nat.PortSet{}
	if port > 0 {
		portStr := nat.Port(fmt.Sprintf("%d/tcp", port))
		exposedPorts[portStr] = struct{}{}
	}

	return types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			ID:   id,
			Name: "/" + name,
		},
		Config: &container.Config{
			Labels:       labels,
			ExposedPorts: exposedPorts,
		},
		NetworkSettings: &types.NetworkSettings{
			Networks: map[string]*network.EndpointSettings{
				networkName: {
					IPAddress: "172.18.0.2",
				},
			},
		},
	}
}

func TestNewClientWithAPI(t *testing.T) {
	mock := &mockDockerAPI{}
	client := NewClientWithAPI(mock, "test-network", "test.localhost")

	if client.networkName != "test-network" {
		t.Errorf("expected networkName 'test-network', got %s", client.networkName)
	}
	if client.baseDomain != "test.localhost" {
		t.Errorf("expected baseDomain 'test.localhost', got %s", client.baseDomain)
	}
}

func TestClient_NetworkName(t *testing.T) {
	mock := &mockDockerAPI{}
	client := NewClientWithAPI(mock, "my-network", "localhost")

	if got := client.NetworkName(); got != "my-network" {
		t.Errorf("NetworkName() = %v, want %v", got, "my-network")
	}
}

func TestClient_BaseDomain(t *testing.T) {
	mock := &mockDockerAPI{}
	client := NewClientWithAPI(mock, "network", "example.localhost")

	if got := client.BaseDomain(); got != "example.localhost" {
		t.Errorf("BaseDomain() = %v, want %v", got, "example.localhost")
	}
}

func TestClient_Close(t *testing.T) {
	mock := &mockDockerAPI{}
	client := NewClientWithAPI(mock, "network", "localhost")

	if err := client.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestClient_DiscoverBackends(t *testing.T) {
	tests := []struct {
		name           string
		networkName    string
		baseDomain     string
		containers     []types.Container
		inspectMap     map[string]types.ContainerJSON
		expectedCount  int
		expectedHosts  []string
	}{
		{
			name:        "single service project",
			networkName: "roji",
			baseDomain:  "localhost",
			containers: []types.Container{
				createMockContainer("abc123", "myproject-web-1", "web", "myproject", 80, "roji"),
			},
			inspectMap: map[string]types.ContainerJSON{
				"abc123": createMockContainerJSON("abc123", "myproject-web-1", "web", "myproject", 80, "roji"),
			},
			expectedCount: 1,
			expectedHosts: []string{"myproject.localhost"},
		},
		{
			name:        "multiple services in project",
			networkName: "roji",
			baseDomain:  "localhost",
			containers: []types.Container{
				createMockContainer("abc123", "myproject-web-1", "web", "myproject", 80, "roji"),
				createMockContainer("def456", "myproject-api-1", "api", "myproject", 3000, "roji"),
			},
			inspectMap: map[string]types.ContainerJSON{
				"abc123": createMockContainerJSON("abc123", "myproject-web-1", "web", "myproject", 80, "roji"),
				"def456": createMockContainerJSON("def456", "myproject-api-1", "api", "myproject", 3000, "roji"),
			},
			expectedCount: 2,
			expectedHosts: []string{"web.myproject.localhost", "api.myproject.localhost"},
		},
		{
			name:          "skip container without port",
			networkName:   "roji",
			baseDomain:    "localhost",
			containers: []types.Container{
				createMockContainer("abc123", "noport-1", "noport", "test", 0, "roji"),
			},
			inspectMap: map[string]types.ContainerJSON{
				"abc123": createMockContainerJSON("abc123", "noport-1", "noport", "test", 0, "roji"),
			},
			expectedCount: 0,
			expectedHosts: []string{},
		},
		{
			name:          "skip roji itself",
			networkName:   "roji",
			baseDomain:    "localhost",
			containers: []types.Container{
				createMockContainer("self123", "roji-dev", "", "", 80, "roji"),
			},
			inspectMap: map[string]types.ContainerJSON{
				"self123": func() types.ContainerJSON {
					ctr := createMockContainerJSON("self123", "roji-dev", "", "", 80, "roji")
					ctr.Config.Labels["roji.self"] = "true"
					return ctr
				}(),
			},
			expectedCount: 0,
			expectedHosts: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockDockerAPI{
				containers: tt.containers,
				inspectMap: tt.inspectMap,
			}
			client := NewClientWithAPI(mock, tt.networkName, tt.baseDomain)

			backends, err := client.DiscoverBackends(context.Background())
			if err != nil {
				t.Fatalf("DiscoverBackends() error = %v", err)
			}

			if len(backends) != tt.expectedCount {
				t.Errorf("DiscoverBackends() got %d backends, want %d", len(backends), tt.expectedCount)
			}

			// Check hostnames
			for i, backend := range backends {
				if i < len(tt.expectedHosts) && backend.Hostname != tt.expectedHosts[i] {
					t.Errorf("Backend[%d] hostname = %v, want %v", i, backend.Hostname, tt.expectedHosts[i])
				}
			}
		})
	}
}

func TestClient_GetBackend(t *testing.T) {
	tests := []struct {
		name        string
		containerID string
		inspectData types.ContainerJSON
		networkName string
		baseDomain  string
		wantBackend bool
		wantHost    string
	}{
		{
			name:        "valid container",
			containerID: "abc123",
			inspectData: createMockContainerJSON("abc123", "web-1", "web", "myproject", 80, "roji"),
			networkName: "roji",
			baseDomain:  "localhost",
			wantBackend: true,
			wantHost:    "myproject.localhost",
		},
		{
			name:        "container not on network",
			containerID: "abc123",
			inspectData: createMockContainerJSON("abc123", "web-1", "web", "myproject", 80, "other-network"),
			networkName: "roji",
			baseDomain:  "localhost",
			wantBackend: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockDockerAPI{
				containerInspect: func(ctx context.Context, containerID string) (types.ContainerJSON, error) {
					return tt.inspectData, nil
				},
			}
			client := NewClientWithAPI(mock, tt.networkName, tt.baseDomain)

			backend, err := client.GetBackend(context.Background(), tt.containerID)
			if err != nil {
				t.Fatalf("GetBackend() error = %v", err)
			}

			if tt.wantBackend && backend == nil {
				t.Error("GetBackend() = nil, want non-nil backend")
			}
			if !tt.wantBackend && backend != nil {
				t.Error("GetBackend() = non-nil, want nil backend")
			}

			if backend != nil && backend.Hostname != tt.wantHost {
				t.Errorf("GetBackend() hostname = %v, want %v", backend.Hostname, tt.wantHost)
			}
		})
	}
}

func TestClient_detectPort(t *testing.T) {
	tests := []struct {
		name     string
		info     types.ContainerJSON
		wantPort int
	}{
		{
			name: "single exposed port",
			info: createMockContainerJSON("abc", "test", "", "", 3000, "roji"),
			wantPort: 3000,
		},
		{
			name: "no exposed port",
			info: createMockContainerJSON("abc", "test", "", "", 0, "roji"),
			wantPort: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockDockerAPI{}
			client := NewClientWithAPI(mock, "roji", "localhost")

			got := client.detectPort(tt.info)
			if got != tt.wantPort {
				t.Errorf("detectPort() = %v, want %v", got, tt.wantPort)
			}
		})
	}
}

func TestClient_GetProjectBackends(t *testing.T) {
	tests := []struct {
		name          string
		projectName   string
		containers    []types.Container
		inspectMap    map[string]types.ContainerJSON
		networkName   string
		baseDomain    string
		expectedCount int
	}{
		{
			name:        "get single project backends",
			projectName: "myproject",
			containers: []types.Container{
				createMockContainer("abc123", "myproject-web-1", "web", "myproject", 80, "roji"),
				createMockContainer("def456", "myproject-api-1", "api", "myproject", 3000, "roji"),
			},
			inspectMap: map[string]types.ContainerJSON{
				"abc123": createMockContainerJSON("abc123", "myproject-web-1", "web", "myproject", 80, "roji"),
				"def456": createMockContainerJSON("def456", "myproject-api-1", "api", "myproject", 3000, "roji"),
			},
			networkName:   "roji",
			baseDomain:    "localhost",
			expectedCount: 2,
		},
		{
			name:        "skip roji.self containers",
			projectName: "myproject",
			containers: []types.Container{
				createMockContainer("abc123", "myproject-web-1", "web", "myproject", 80, "roji"),
			},
			inspectMap: map[string]types.ContainerJSON{
				"abc123": func() types.ContainerJSON {
					ctr := createMockContainerJSON("abc123", "myproject-web-1", "web", "myproject", 80, "roji")
					ctr.Config.Labels["roji.self"] = "true"
					return ctr
				}(),
			},
			networkName:   "roji",
			baseDomain:    "localhost",
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockDockerAPI{
				containers: tt.containers,
				inspectMap: tt.inspectMap,
			}
			client := NewClientWithAPI(mock, tt.networkName, tt.baseDomain)

			backends, err := client.GetProjectBackends(context.Background(), tt.projectName)
			if err != nil {
				t.Fatalf("GetProjectBackends() error = %v", err)
			}

			if len(backends) != tt.expectedCount {
				t.Errorf("GetProjectBackends() got %d backends, want %d", len(backends), tt.expectedCount)
			}
		})
	}
}

func TestClient_DockerClient(t *testing.T) {
	mock := &mockDockerAPI{}
	client := NewClientWithAPI(mock, "network", "localhost")

	dockerClient := client.DockerClient()
	if dockerClient != mock {
		t.Error("DockerClient() did not return the expected API instance")
	}
}

func TestClient_detectHostname(t *testing.T) {
	tests := []struct {
		name                string
		info                types.ContainerJSON
		projectServiceCount map[string]int
		baseDomain          string
		wantHostname        string
	}{
		{
			name:                "single service in project",
			info:                createMockContainerJSON("abc", "myproject-web-1", "web", "myproject", 80, "roji"),
			projectServiceCount: map[string]int{"myproject": 1},
			baseDomain:          "localhost",
			wantHostname:        "myproject.localhost",
		},
		{
			name:                "multiple services in project",
			info:                createMockContainerJSON("abc", "myproject-web-1", "web", "myproject", 80, "roji"),
			projectServiceCount: map[string]int{"myproject": 2},
			baseDomain:          "localhost",
			wantHostname:        "web.myproject.localhost",
		},
		{
			name:                "non-compose container",
			info:                createMockContainerJSON("abc", "standalone-app", "", "", 80, "roji"),
			projectServiceCount: map[string]int{},
			baseDomain:          "localhost",
			wantHostname:        "standalone-app.localhost",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockDockerAPI{}
			client := NewClientWithAPI(mock, "roji", tt.baseDomain)

			got := client.detectHostname(tt.info, tt.projectServiceCount)
			if got != tt.wantHostname {
				t.Errorf("detectHostname() = %v, want %v", got, tt.wantHostname)
			}
		})
	}
}
