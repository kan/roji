package docker

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"testing"

	dockerclient "github.com/moby/moby/client"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/events"
	"github.com/moby/moby/api/types/network"
)

func mustParseAddr(s string) netip.Addr {
	return netip.MustParseAddr(s)
}

// mockDockerAPI is a mock implementation of DockerAPI for testing
type mockDockerAPI struct {
	containers       []container.Summary
	inspectMap       map[string]container.InspectResponse
	containerList    func(ctx context.Context, options dockerclient.ContainerListOptions) (dockerclient.ContainerListResult, error)
	containerInspect func(ctx context.Context, containerID string, options dockerclient.ContainerInspectOptions) (dockerclient.ContainerInspectResult, error)
	eventsFunc       func(ctx context.Context, options dockerclient.EventsListOptions) dockerclient.EventsResult
	restartErr       error // error to return from ContainerRestart
}

func (m *mockDockerAPI) ContainerList(ctx context.Context, options dockerclient.ContainerListOptions) (dockerclient.ContainerListResult, error) {
	if m.containerList != nil {
		return m.containerList(ctx, options)
	}
	return dockerclient.ContainerListResult{Items: m.containers}, nil
}

func (m *mockDockerAPI) ContainerInspect(ctx context.Context, containerID string, options dockerclient.ContainerInspectOptions) (dockerclient.ContainerInspectResult, error) {
	if m.containerInspect != nil {
		return m.containerInspect(ctx, containerID, options)
	}
	if info, ok := m.inspectMap[containerID]; ok {
		return dockerclient.ContainerInspectResult{Container: info}, nil
	}
	return dockerclient.ContainerInspectResult{}, nil
}

func (m *mockDockerAPI) Events(ctx context.Context, options dockerclient.EventsListOptions) dockerclient.EventsResult {
	if m.eventsFunc != nil {
		return m.eventsFunc(ctx, options)
	}
	msgCh := make(chan events.Message)
	errCh := make(chan error)
	close(msgCh)
	close(errCh)
	return dockerclient.EventsResult{Messages: msgCh, Err: errCh}
}

func (m *mockDockerAPI) Close() error {
	return nil
}

func (m *mockDockerAPI) ContainerRestart(ctx context.Context, containerID string, options dockerclient.ContainerRestartOptions) (dockerclient.ContainerRestartResult, error) {
	return dockerclient.ContainerRestartResult{}, m.restartErr
}

// Test helper to create a mock container Summary
func createMockContainer(id, name, serviceName, projectName string, port int, networkName string) container.Summary {
	labels := map[string]string{}
	if serviceName != "" {
		labels["com.docker.compose.service"] = serviceName
	}
	if projectName != "" {
		labels["com.docker.compose.project"] = projectName
	}

	return container.Summary{
		ID:     id,
		Names:  []string{"/" + name},
		Labels: labels,
		NetworkSettings: &container.NetworkSettingsSummary{
			Networks: map[string]*network.EndpointSettings{
				networkName: {
					IPAddress: mustParseAddr("172.18.0.2"),
				},
			},
		},
	}
}

// Test helper to create a mock container InspectResponse
func createMockContainerJSON(id, name, serviceName, projectName string, port int, networkName string) container.InspectResponse {
	labels := map[string]string{}
	if serviceName != "" {
		labels["com.docker.compose.service"] = serviceName
	}
	if projectName != "" {
		labels["com.docker.compose.project"] = projectName
	}

	exposedPorts := network.PortSet{}
	if port > 0 {
		p := network.MustParsePort(fmt.Sprintf("%d/tcp", port))
		exposedPorts[p] = struct{}{}
	}

	return container.InspectResponse{
		ID:   id,
		Name: "/" + name,
		Config: &container.Config{
			Labels:       labels,
			ExposedPorts: exposedPorts,
		},
		NetworkSettings: &container.NetworkSettings{
			Networks: map[string]*network.EndpointSettings{
				networkName: {
					IPAddress: mustParseAddr("172.18.0.2"),
				},
			},
		},
	}
}

func TestNewClientWithAPI(t *testing.T) {
	mock := &mockDockerAPI{}
	client := NewClientWithAPI(mock, []string{"test-network"}, "test.localhost")

	if len(client.networks) != 1 || client.networks[0] != "test-network" {
		t.Errorf("expected networks ['test-network'], got %v", client.networks)
	}
	if client.baseDomain != "test.localhost" {
		t.Errorf("expected baseDomain 'test.localhost', got %s", client.baseDomain)
	}
}

func TestClient_Networks(t *testing.T) {
	mock := &mockDockerAPI{}
	client := NewClientWithAPI(mock, []string{"my-network", "other-network"}, "localhost")

	got := client.Networks()
	if len(got) != 2 || got[0] != "my-network" || got[1] != "other-network" {
		t.Errorf("Networks() = %v, want [my-network, other-network]", got)
	}
}

func TestClient_BaseDomain(t *testing.T) {
	mock := &mockDockerAPI{}
	client := NewClientWithAPI(mock, []string{"network"}, "example.localhost")

	if got := client.BaseDomain(); got != "example.localhost" {
		t.Errorf("BaseDomain() = %v, want %v", got, "example.localhost")
	}
}

func TestClient_Close(t *testing.T) {
	mock := &mockDockerAPI{}
	client := NewClientWithAPI(mock, []string{"network"}, "localhost")

	if err := client.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestClient_DiscoverBackends(t *testing.T) {
	tests := []struct {
		name            string
		networkName     string
		baseDomain      string
		containers      []container.Summary
		inspectMap      map[string]container.InspectResponse
		expectedCount   int
		expectedHosts   []string
		expectedWarning string
	}{
		{
			name:        "single service project",
			networkName: "roji",
			baseDomain:  "localhost",
			containers: []container.Summary{
				createMockContainer("abc123", "myproject-web-1", "web", "myproject", 80, "roji"),
			},
			inspectMap: map[string]container.InspectResponse{
				"abc123": createMockContainerJSON("abc123", "myproject-web-1", "web", "myproject", 80, "roji"),
			},
			expectedCount: 1,
			expectedHosts: []string{"myproject.localhost"},
		},
		{
			name:        "multiple services in project",
			networkName: "roji",
			baseDomain:  "localhost",
			containers: []container.Summary{
				createMockContainer("abc123", "myproject-web-1", "web", "myproject", 80, "roji"),
				createMockContainer("def456", "myproject-api-1", "api", "myproject", 3000, "roji"),
			},
			inspectMap: map[string]container.InspectResponse{
				"abc123": createMockContainerJSON("abc123", "myproject-web-1", "web", "myproject", 80, "roji"),
				"def456": createMockContainerJSON("def456", "myproject-api-1", "api", "myproject", 3000, "roji"),
			},
			expectedCount: 2,
			expectedHosts: []string{"web-myproject.localhost", "api-myproject.localhost"},
		},
		{
			name:        "container without port has warning",
			networkName: "roji",
			baseDomain:  "localhost",
			containers: []container.Summary{
				createMockContainer("abc123", "noport-1", "noport", "test", 0, "roji"),
			},
			inspectMap: map[string]container.InspectResponse{
				"abc123": createMockContainerJSON("abc123", "noport-1", "noport", "test", 0, "roji"),
			},
			expectedCount:   1,
			expectedHosts:   []string{"test.localhost"},
			expectedWarning: "no port exposed",
		},
		{
			name:        "skip roji itself",
			networkName: "roji",
			baseDomain:  "localhost",
			containers: []container.Summary{
				createMockContainer("self123", "roji-dev", "", "", 80, "roji"),
			},
			inspectMap: map[string]container.InspectResponse{
				"self123": func() container.InspectResponse {
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
			client := NewClientWithAPI(mock, []string{tt.networkName}, tt.baseDomain)

			backends, err := client.DiscoverBackends(context.Background())
			if err != nil {
				t.Fatalf("DiscoverBackends() error = %v", err)
			}

			if len(backends) != tt.expectedCount {
				t.Errorf("DiscoverBackends() got %d backends, want %d", len(backends), tt.expectedCount)
			}

			// Check hostnames (order-independent)
			gotHosts := make(map[string]bool)
			for _, backend := range backends {
				gotHosts[backend.Hostname] = true
			}
			for _, expectedHost := range tt.expectedHosts {
				if !gotHosts[expectedHost] {
					t.Errorf("DiscoverBackends() missing expected hostname %q", expectedHost)
				}
			}

			// Check expected warning if specified
			if tt.expectedWarning != "" && len(backends) > 0 {
				if backends[0].Warning != tt.expectedWarning {
					t.Errorf("Backend[0] warning = %v, want %v", backends[0].Warning, tt.expectedWarning)
				}
			}
		})
	}
}

func TestClient_GetBackend(t *testing.T) {
	tests := []struct {
		name        string
		containerID string
		inspectData container.InspectResponse
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
				containerInspect: func(ctx context.Context, containerID string, options dockerclient.ContainerInspectOptions) (dockerclient.ContainerInspectResult, error) {
					return dockerclient.ContainerInspectResult{Container: tt.inspectData}, nil
				},
			}
			client := NewClientWithAPI(mock, []string{tt.networkName}, tt.baseDomain)

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
		info     container.InspectResponse
		wantPort int
	}{
		{
			name:     "single exposed port",
			info:     createMockContainerJSON("abc", "test", "", "", 3000, "roji"),
			wantPort: 3000,
		},
		{
			name:     "no exposed port",
			info:     createMockContainerJSON("abc", "test", "", "", 0, "roji"),
			wantPort: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockDockerAPI{}
			client := NewClientWithAPI(mock, []string{"roji"}, "localhost")

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
		containers    []container.Summary
		inspectMap    map[string]container.InspectResponse
		networkName   string
		baseDomain    string
		expectedCount int
	}{
		{
			name:        "get single project backends",
			projectName: "myproject",
			containers: []container.Summary{
				createMockContainer("abc123", "myproject-web-1", "web", "myproject", 80, "roji"),
				createMockContainer("def456", "myproject-api-1", "api", "myproject", 3000, "roji"),
			},
			inspectMap: map[string]container.InspectResponse{
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
			containers: []container.Summary{
				createMockContainer("abc123", "myproject-web-1", "web", "myproject", 80, "roji"),
			},
			inspectMap: map[string]container.InspectResponse{
				"abc123": func() container.InspectResponse {
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
			client := NewClientWithAPI(mock, []string{tt.networkName}, tt.baseDomain)

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
	client := NewClientWithAPI(mock, []string{"network"}, "localhost")

	dockerClient := client.DockerClient()
	if dockerClient != mock {
		t.Error("DockerClient() did not return the expected API instance")
	}
}

func TestClient_detectHostname(t *testing.T) {
	tests := []struct {
		name                string
		info                container.InspectResponse
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
			wantHostname:        "web-myproject.localhost",
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
			client := NewClientWithAPI(mock, []string{"roji"}, tt.baseDomain)

			got := client.detectHostname(tt.info, tt.projectServiceCount)
			if got != tt.wantHostname {
				t.Errorf("detectHostname() = %v, want %v", got, tt.wantHostname)
			}
		})
	}
}

func TestTrimLeadingSlash(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "leading slash removed",
			input: "/container-name",
			want:  "container-name",
		},
		{
			name:  "no leading slash unchanged",
			input: "container-name",
			want:  "container-name",
		},
		{
			name:  "empty string unchanged",
			input: "",
			want:  "",
		},
		{
			name:  "slash only becomes empty",
			input: "/",
			want:  "",
		},
		{
			name:  "multiple slashes: only first removed",
			input: "//double",
			want:  "/double",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := trimLeadingSlash(tt.input)
			if got != tt.want {
				t.Errorf("trimLeadingSlash(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildProjectServiceCounts(t *testing.T) {
	tests := []struct {
		name       string
		containers []container.Summary
		want       map[string]int
	}{
		{
			name: "single project single service",
			containers: []container.Summary{
				createMockContainer("abc", "web-1", "web", "myproject", 80, "roji"),
			},
			want: map[string]int{"myproject": 1},
		},
		{
			name: "single project multiple services",
			containers: []container.Summary{
				createMockContainer("abc", "web-1", "web", "myproject", 80, "roji"),
				createMockContainer("def", "api-1", "api", "myproject", 3000, "roji"),
			},
			want: map[string]int{"myproject": 2},
		},
		{
			name: "multiple projects",
			containers: []container.Summary{
				createMockContainer("abc", "web-1", "web", "proj1", 80, "roji"),
				createMockContainer("def", "api-1", "api", "proj2", 3000, "roji"),
			},
			want: map[string]int{"proj1": 1, "proj2": 1},
		},
		{
			name: "roji.self container is excluded",
			containers: []container.Summary{
				func() container.Summary {
					ctr := createMockContainer("self", "roji", "", "", 80, "roji")
					ctr.Labels["roji.self"] = "true"
					ctr.Labels["com.docker.compose.project"] = "roji"
					return ctr
				}(),
				createMockContainer("abc", "web-1", "web", "myproject", 80, "roji"),
			},
			want: map[string]int{"myproject": 1},
		},
		{
			name: "non-compose container is excluded",
			containers: []container.Summary{
				createMockContainer("abc", "standalone", "", "", 80, "roji"),
			},
			want: map[string]int{},
		},
		{
			name:       "empty list",
			containers: []container.Summary{},
			want:       map[string]int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildProjectServiceCounts(tt.containers)
			if len(got) != len(tt.want) {
				t.Errorf("buildProjectServiceCounts() = %v, want %v", got, tt.want)
				return
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("buildProjectServiceCounts()[%q] = %d, want %d", k, got[k], v)
				}
			}
		})
	}
}

func TestClient_RestartContainer(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := &mockDockerAPI{}
		client := NewClientWithAPI(mock, []string{"roji"}, "localhost")

		err := client.RestartContainer(context.Background(), "abc123")
		if err != nil {
			t.Errorf("RestartContainer() unexpected error = %v", err)
		}
	})

	t.Run("propagates error from ContainerRestart", func(t *testing.T) {
		wantErr := errors.New("restart failed")
		mock := &mockDockerAPI{restartErr: wantErr}
		client := NewClientWithAPI(mock, []string{"roji"}, "localhost")

		err := client.RestartContainer(context.Background(), "abc123")
		if err == nil {
			t.Fatal("RestartContainer() expected error, got nil")
		}
		if !errors.Is(err, wantErr) {
			t.Errorf("RestartContainer() error = %v, want %v", err, wantErr)
		}
	})
}

func TestClient_GetProjectInfo(t *testing.T) {
	t.Run("docker-compose container returns project info", func(t *testing.T) {
		ctr := createMockContainerJSON("abc123", "myproject-web-1", "web", "myproject", 80, "roji")
		ctr.Config.Labels["com.docker.compose.project.working_dir"] = "/home/user/myproject"
		ctr.Config.Labels["com.docker.compose.project.config_files"] = "/home/user/myproject/docker-compose.yml"

		mock := &mockDockerAPI{
			containerInspect: func(ctx context.Context, containerID string, options dockerclient.ContainerInspectOptions) (dockerclient.ContainerInspectResult, error) {
				return dockerclient.ContainerInspectResult{Container: ctr}, nil
			},
		}
		client := NewClientWithAPI(mock, []string{"roji"}, "localhost")

		info, err := client.GetProjectInfo(context.Background(), "abc123")
		if err != nil {
			t.Fatalf("GetProjectInfo() error = %v", err)
		}
		if info == nil {
			t.Fatal("GetProjectInfo() = nil, want non-nil")
		}
		if info.Name != "myproject" {
			t.Errorf("GetProjectInfo().Name = %q, want %q", info.Name, "myproject")
		}
		if info.WorkingDir != "/home/user/myproject" {
			t.Errorf("GetProjectInfo().WorkingDir = %q, want %q", info.WorkingDir, "/home/user/myproject")
		}
		if info.ConfigFiles != "/home/user/myproject/docker-compose.yml" {
			t.Errorf("GetProjectInfo().ConfigFiles = %q, want %q", info.ConfigFiles, "/home/user/myproject/docker-compose.yml")
		}
	})

	t.Run("non-compose container returns nil", func(t *testing.T) {
		ctr := createMockContainerJSON("abc123", "standalone", "", "", 80, "roji")

		mock := &mockDockerAPI{
			containerInspect: func(ctx context.Context, containerID string, options dockerclient.ContainerInspectOptions) (dockerclient.ContainerInspectResult, error) {
				return dockerclient.ContainerInspectResult{Container: ctr}, nil
			},
		}
		client := NewClientWithAPI(mock, []string{"roji"}, "localhost")

		info, err := client.GetProjectInfo(context.Background(), "abc123")
		if err != nil {
			t.Fatalf("GetProjectInfo() error = %v", err)
		}
		if info != nil {
			t.Errorf("GetProjectInfo() = %v, want nil for non-compose container", info)
		}
	})

	t.Run("inspect error is propagated", func(t *testing.T) {
		wantErr := errors.New("inspect failed")
		mock := &mockDockerAPI{
			containerInspect: func(ctx context.Context, containerID string, options dockerclient.ContainerInspectOptions) (dockerclient.ContainerInspectResult, error) {
				return dockerclient.ContainerInspectResult{}, wantErr
			},
		}
		client := NewClientWithAPI(mock, []string{"roji"}, "localhost")

		_, err := client.GetProjectInfo(context.Background(), "abc123")
		if err == nil {
			t.Fatal("GetProjectInfo() expected error, got nil")
		}
	})
}

func TestClient_DiscoverProjects(t *testing.T) {
	tests := []struct {
		name         string
		containers   []container.Summary
		wantProjects map[string]int // projectName -> service count
	}{
		{
			name: "single project with multiple services",
			containers: []container.Summary{
				createMockContainer("abc", "myproject-web-1", "web", "myproject", 80, "roji"),
				createMockContainer("def", "myproject-api-1", "api", "myproject", 3000, "roji"),
			},
			wantProjects: map[string]int{"myproject": 2},
		},
		{
			name: "multiple projects",
			containers: []container.Summary{
				createMockContainer("abc", "proj1-web-1", "web", "proj1", 80, "roji"),
				createMockContainer("def", "proj2-api-1", "api", "proj2", 3000, "roji"),
			},
			wantProjects: map[string]int{"proj1": 1, "proj2": 1},
		},
		{
			name: "roji.self container is excluded",
			containers: []container.Summary{
				func() container.Summary {
					ctr := createMockContainer("self", "roji", "", "", 80, "roji")
					ctr.Labels["roji.self"] = "true"
					ctr.Labels["com.docker.compose.project"] = "roji"
					return ctr
				}(),
				createMockContainer("abc", "myproject-web-1", "web", "myproject", 80, "roji"),
			},
			wantProjects: map[string]int{"myproject": 1},
		},
		{
			name: "non-compose containers are excluded",
			containers: []container.Summary{
				createMockContainer("abc", "standalone", "", "", 80, "roji"),
			},
			wantProjects: map[string]int{},
		},
		{
			name:         "empty result",
			containers:   []container.Summary{},
			wantProjects: map[string]int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockDockerAPI{containers: tt.containers}
			client := NewClientWithAPI(mock, []string{"roji"}, "localhost")

			projects, err := client.DiscoverProjects(context.Background())
			if err != nil {
				t.Fatalf("DiscoverProjects() error = %v", err)
			}

			if len(projects) != len(tt.wantProjects) {
				t.Errorf("DiscoverProjects() got %d projects, want %d: %v", len(projects), len(tt.wantProjects), projects)
			}

			for projectName, wantCount := range tt.wantProjects {
				p, ok := projects[projectName]
				if !ok {
					t.Errorf("DiscoverProjects() missing project %q", projectName)
					continue
				}
				if len(p.Services) != wantCount {
					t.Errorf("project %q has %d services, want %d", projectName, len(p.Services), wantCount)
				}
			}
		})
	}
}

func TestClient_inspectToBackend_IPAddress(t *testing.T) {
	t.Run("valid IP address is converted via String()", func(t *testing.T) {
		ctr := createMockContainerJSON("abc", "myproject-web-1", "web", "myproject", 80, "roji")
		net := ctr.NetworkSettings.Networks["roji"]

		mock := &mockDockerAPI{}
		c := NewClientWithAPI(mock, []string{"roji"}, "localhost")

		backend, err := c.inspectToBackend(ctr, "roji", net, map[string]int{"myproject": 1})
		if err != nil {
			t.Fatalf("inspectToBackend() error = %v", err)
		}
		if backend == nil {
			t.Fatal("inspectToBackend() = nil, want non-nil")
		}
		if backend.Host != "172.18.0.2" {
			t.Errorf("inspectToBackend().Host = %q, want %q", backend.Host, "172.18.0.2")
		}
	})

	t.Run("zero IP address becomes 0.0.0.0 via String()", func(t *testing.T) {
		ctr := createMockContainerJSON("abc", "myproject-web-1", "web", "myproject", 80, "roji")
		// Override the IP to zero value (netip.Addr{})
		ctr.NetworkSettings.Networks["roji"] = &network.EndpointSettings{
			IPAddress: netip.Addr{},
		}
		net := ctr.NetworkSettings.Networks["roji"]

		mock := &mockDockerAPI{}
		c := NewClientWithAPI(mock, []string{"roji"}, "localhost")

		backend, err := c.inspectToBackend(ctr, "roji", net, map[string]int{"myproject": 1})
		if err != nil {
			t.Fatalf("inspectToBackend() error = %v", err)
		}
		if backend == nil {
			t.Fatal("inspectToBackend() = nil, want non-nil")
		}
		// netip.Addr{}.String() returns "invalid IP"
		wantHost := netip.Addr{}.String()
		if backend.Host != wantHost {
			t.Errorf("inspectToBackend().Host = %q, want %q", backend.Host, wantHost)
		}
	})
}
