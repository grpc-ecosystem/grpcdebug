package cmd

import (
	"strings"
	"testing"

	csdspb "github.com/envoyproxy/go-control-plane/envoy/service/status/v3"
)

func TestFilterConfigsByScope(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		configs    []*csdspb.ClientConfig
		scope      string
		wantLen    int
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "empty scope returns all",
			configs: []*csdspb.ClientConfig{
				{ClientScope: "primary"},
				{ClientScope: "fallback"},
			},
			scope:   "",
			wantLen: 2,
			wantErr: false,
		},
		{
			name: "filter by primary scope",
			configs: []*csdspb.ClientConfig{
				{ClientScope: "primary"},
				{ClientScope: "fallback"},
			},
			scope:   "primary",
			wantLen: 1,
			wantErr: false,
		},
		{
			name: "filter by fallback scope",
			configs: []*csdspb.ClientConfig{
				{ClientScope: "primary"},
				{ClientScope: "fallback"},
			},
			scope:   "fallback",
			wantLen: 1,
			wantErr: false,
		},
		{
			name: "no match returns error",
			configs: []*csdspb.ClientConfig{
				{ClientScope: "primary"},
				{ClientScope: "fallback"},
			},
			scope:      "nonexistent",
			wantLen:    0,
			wantErr:    true,
			wantErrMsg: "no ClientConfig matched scope=",
		},
		{
			name: "multiple configs with same scope",
			configs: []*csdspb.ClientConfig{
				{ClientScope: "primary"},
				{ClientScope: "primary"},
				{ClientScope: "fallback"},
			},
			scope:   "primary",
			wantLen: 2,
			wantErr: false,
		},
		{
			name: "empty scope name in config",
			configs: []*csdspb.ClientConfig{
				{ClientScope: ""},
				{ClientScope: "primary"},
			},
			scope:   "",
			wantLen: 2,
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := filterConfigsByScope(test.configs, test.scope)
			if test.wantErr {
				if err == nil {
					t.Errorf("filterConfigsByScope() expected error but got none")
					return
				}
				if !strings.Contains(err.Error(), test.wantErrMsg) {
					t.Errorf("filterConfigsByScope() error = %v, want error containing %v", err, test.wantErrMsg)
				}
				return
			}
			if err != nil {
				t.Errorf("filterConfigsByScope() unexpected error = %v", err)
				return
			}
			if len(got) != test.wantLen {
				t.Errorf("filterConfigsByScope() returned %d configs, want %d", len(got), test.wantLen)
			}
			// Verify filtered results actually match the scope
			if test.scope != "" {
				for _, c := range got {
					if c.ClientScope != test.scope {
						t.Errorf("filterConfigsByScope() returned config with scope %q, want %q", c.ClientScope, test.scope)
					}
				}
			}
		})
	}
}

func TestSortPerXdsConfigs(t *testing.T) {
	t.Parallel()
	// Test that sortPerXdsConfigs doesn't panic with multiple configs
	// XdsConfig is deprecated but we test it for backward compatibility
	clientStatus := &csdspb.ClientStatusResponse{
		Config: []*csdspb.ClientConfig{
			{
				ClientScope: "primary",
				XdsConfig: []*csdspb.PerXdsConfig{
					{PerXdsConfig: &csdspb.PerXdsConfig_ClusterConfig{}},
					{PerXdsConfig: &csdspb.PerXdsConfig_ListenerConfig{}},
				},
			},
			{
				ClientScope: "fallback",
				XdsConfig: []*csdspb.PerXdsConfig{
					{PerXdsConfig: &csdspb.PerXdsConfig_EndpointConfig{}},
					{PerXdsConfig: &csdspb.PerXdsConfig_RouteConfig{}},
				},
			},
		},
	}

	// Should not panic
	sortPerXdsConfigs(clientStatus)

	// Verify first config is sorted: Listener(0) < Cluster(2)
	if len(clientStatus.Config[0].XdsConfig) >= 2 {
		p0 := priorityPerXdsConfig(clientStatus.Config[0].XdsConfig[0])
		p1 := priorityPerXdsConfig(clientStatus.Config[0].XdsConfig[1])
		if p0 > p1 {
			t.Errorf("First config not sorted properly: priority[0]=%d > priority[1]=%d", p0, p1)
		}
	}

	// Verify second config is sorted: Route(1) < Endpoint(3)
	if len(clientStatus.Config[1].XdsConfig) >= 2 {
		p0 := priorityPerXdsConfig(clientStatus.Config[1].XdsConfig[0])
		p1 := priorityPerXdsConfig(clientStatus.Config[1].XdsConfig[1])
		if p0 > p1 {
			t.Errorf("Second config not sorted properly: priority[0]=%d > priority[1]=%d", p0, p1)
		}
	}
}

func TestSortPerXdsConfigsEmptyConfigs(t *testing.T) {
	t.Parallel()
	// Test with empty configs
	clientStatus := &csdspb.ClientStatusResponse{
		Config: []*csdspb.ClientConfig{},
	}
	sortPerXdsConfigs(clientStatus)
}

func TestSortPerXdsConfigsSingleConfig(t *testing.T) {
	t.Parallel()
	// Test backward compatibility with single config
	// XdsConfig is deprecated but we test it for backward compatibility
	clientStatus := &csdspb.ClientStatusResponse{
		Config: []*csdspb.ClientConfig{
			{
				ClientScope: "",
				XdsConfig: []*csdspb.PerXdsConfig{
					{PerXdsConfig: &csdspb.PerXdsConfig_EndpointConfig{}},
					{PerXdsConfig: &csdspb.PerXdsConfig_ListenerConfig{}},
					{PerXdsConfig: &csdspb.PerXdsConfig_ClusterConfig{}},
				},
			},
		},
	}

	sortPerXdsConfigs(clientStatus)

	// Verify sorted: Listener(0) < Cluster(2) < Endpoint(3)
	expected := []int{0, 2, 3}
	for i, cfg := range clientStatus.Config[0].XdsConfig {
		priority := priorityPerXdsConfig(cfg)
		if priority != expected[i] {
			t.Errorf("Config[%d] priority = %d, want %d", i, priority, expected[i])
		}
	}
}
