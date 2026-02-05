package docker

import (
	"reflect"
	"testing"
)

func TestBuildComposeArgs(t *testing.T) {
	tests := []struct {
		name        string
		configFiles string
		args        []string
		want        []string
	}{
		{
			name:        "empty config files",
			configFiles: "",
			args:        []string{"up", "-d"},
			want:        []string{"compose", "up", "-d"},
		},
		{
			name:        "single config file",
			configFiles: "docker-compose.yml",
			args:        []string{"up", "-d"},
			want:        []string{"compose", "-f", "docker-compose.yml", "up", "-d"},
		},
		{
			name:        "multiple config files",
			configFiles: "docker-compose.yml,docker-compose.dev.yml",
			args:        []string{"down"},
			want:        []string{"compose", "-f", "docker-compose.yml", "-f", "docker-compose.dev.yml", "down"},
		},
		{
			name:        "config files with spaces",
			configFiles: "docker-compose.yml, docker-compose.override.yml",
			args:        []string{"restart"},
			want:        []string{"compose", "-f", "docker-compose.yml", "-f", "docker-compose.override.yml", "restart"},
		},
		{
			name:        "no additional args",
			configFiles: "",
			args:        []string{},
			want:        []string{"compose"},
		},
		{
			name:        "logs with follow",
			configFiles: "docker-compose.yml",
			args:        []string{"logs", "-f", "--no-color"},
			want:        []string{"compose", "-f", "docker-compose.yml", "logs", "-f", "--no-color"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildComposeArgs(tt.configFiles, tt.args...)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildComposeArgs(%q, %v) = %v, want %v", tt.configFiles, tt.args, got, tt.want)
			}
		})
	}
}
