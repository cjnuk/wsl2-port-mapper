package main

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestPortExternalPortEffective(t *testing.T) {
	tests := []struct {
		name     string
		port     Port
		expected int
	}{
		{
			name:     "Simple port only",
			port:     Port{Port: 8080},
			expected: 8080,
		},
		{
			name:     "Port with same internal port",
			port:     Port{Port: 8080, InternalPort: 8080},
			expected: 8080,
		},
		{
			name:     "Port with different internal port",
			port:     Port{Port: 8080, InternalPort: 80},
			expected: 8080,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.port.ExternalPortEffective(); got != tt.expected {
				t.Errorf("ExternalPortEffective() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPortInternalPortEffective(t *testing.T) {
	tests := []struct {
		name     string
		port     Port
		expected int
	}{
		{
			name:     "Simple port only, should default to external",
			port:     Port{Port: 8080},
			expected: 8080,
		},
		{
			name:     "Port with same internal port",
			port:     Port{Port: 8080, InternalPort: 8080},
			expected: 8080,
		},
		{
			name:     "Port with different internal port",
			port:     Port{Port: 8080, InternalPort: 80},
			expected: 80,
		},
		{
			name:     "Internal port zero should default to external",
			port:     Port{Port: 8080, InternalPort: 0},
			expected: 8080,
		},
		{
			name:     "SSH mapping example",
			port:     Port{Port: 2201, InternalPort: 22},
			expected: 22,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.port.InternalPortEffective(); got != tt.expected {
				t.Errorf("InternalPortEffective() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestValidationAllowDuplicateExternalPorts(t *testing.T) {
	service := &ServiceState{}

	config := &Config{
		CheckIntervalSeconds: 5,
		Instances: []Instance{
			{
				Name: "Ubuntu-1",
				Ports: []Port{
					{Port: 8080, InternalPort: 80},
				},
			},
			{
				Name: "Ubuntu-2",
				Ports: []Port{
					{Port: 8080, InternalPort: 8080}, // Duplicate external port should be allowed
				},
			},
		},
	}

	err := service.validateConfiguration(config)
	if err != nil {
		t.Errorf("Expected no validation error for duplicate external ports, got: %v", err)
	}
}

func TestValidationAllowDuplicateInternalPorts(t *testing.T) {
	service := &ServiceState{}

	config := &Config{
		CheckIntervalSeconds: 5,
		Instances: []Instance{
			{
				Name: "Ubuntu-1",
				Ports: []Port{
					{Port: 2201, InternalPort: 22}, // SSH on external 2201
				},
			},
			{
				Name: "Ubuntu-2",
				Ports: []Port{
					{Port: 2202, InternalPort: 22}, // SSH on external 2202, same internal port is OK
				},
			},
		},
	}

	err := service.validateConfiguration(config)
	if err != nil {
		t.Errorf("Expected no validation error for duplicate internal ports, got: %v", err)
	}
}

func TestValidationInvalidInternalPort(t *testing.T) {
	service := &ServiceState{}

	config := &Config{
		CheckIntervalSeconds: 5,
		Instances: []Instance{
			{
				Name: "Ubuntu-1",
				Ports: []Port{
					{Port: 8080, InternalPort: 70000}, // Invalid internal port
				},
			},
		},
	}

	err := service.validateConfiguration(config)
	if err == nil {
		t.Error("Expected validation error for invalid internal port, got nil")
	}
	if err != nil && !contains(err.Error(), "invalid internal port") {
		t.Errorf("Expected error about invalid internal port, got: %v", err)
	}
}

func TestValidationValidInternalPortZero(t *testing.T) {
	service := &ServiceState{}

	config := &Config{
		CheckIntervalSeconds: 5,
		Instances: []Instance{
			{
				Name: "Ubuntu-1",
				Ports: []Port{
					{Port: 8080, InternalPort: 0}, // Zero internal port should be valid (defaults to external)
				},
			},
		},
	}

	err := service.validateConfiguration(config)
	if err != nil {
		t.Errorf("Expected no validation error for zero internal port, got: %v", err)
	}
}

func TestRuntimeConflictResolution(t *testing.T) {
	// This test would require mocking the running instances
	// For now, we test that the validation allows duplicates
	service := &ServiceState{}

	config := &Config{
		CheckIntervalSeconds: 5,
		Instances: []Instance{
			{
				Name: "Ubuntu-Dev",
				Ports: []Port{
					{Port: 2222, InternalPort: 22},
				},
			},
			{
				Name: "Ubuntu-Prod",
				Ports: []Port{
					{Port: 2222, InternalPort: 22}, // Same external port, different instance
				},
			},
		},
	}

	// Should validate successfully
	err := service.validateConfiguration(config)
	if err != nil {
		t.Errorf("Expected no validation error for duplicate external ports in different instances, got: %v", err)
	}
}

func TestValidateOnlyMode(t *testing.T) {
	// Create a temporary config file
	tempConfig := `{
		"check_interval_seconds": 5,
		"instances": [
			{
				"name": "Test-Instance",
				"ports": [
					{"port": 8080, "internal_port": 80}
				]
			}
		]
	}`

	// Note: This test would need file system mocking to be fully testable
	// For now, we just test the config validation logic
	service := &ServiceState{}
	var config Config
	err := json.Unmarshal([]byte(tempConfig), &config)
	if err != nil {
		t.Fatalf("Failed to unmarshal test config: %v", err)
	}

	err = service.validateConfiguration(&config)
	if err != nil {
		t.Errorf("Expected valid config, got error: %v", err)
	}
}

func TestValidationPortConflictWarning(t *testing.T) {
	// Test that we can detect potential port conflicts during validation
	config := &Config{
		CheckIntervalSeconds: 5,
		Instances: []Instance{
			{
				Name: "Instance-A",
				Ports: []Port{
					{Port: 3000, InternalPort: 3000},
				},
			},
			{
				Name: "Instance-B",
				Ports: []Port{
					{Port: 3000, InternalPort: 3001}, // Same external port
				},
			},
		},
	}

	// Count potential conflicts
	portToInstances := make(map[int][]string)
	for _, instance := range config.Instances {
		for _, port := range instance.Ports {
			externalPort := port.ExternalPortEffective()
			portToInstances[externalPort] = append(portToInstances[externalPort], instance.Name)
		}
	}

	conflicts := 0
	for _, instances := range portToInstances {
		if len(instances) > 1 {
			conflicts++
		}
	}

	if conflicts != 1 {
		t.Errorf("Expected 1 port conflict, found %d", conflicts)
	}
}

func TestPortFirewallMethods(t *testing.T) {
	tests := []struct {
		name           string
		port           Port
		expectedMode   string
		expectedManage bool
	}{
		{
			name:           "No firewall setting",
			port:           Port{Port: 8080},
			expectedMode:   "",
			expectedManage: false,
		},
		{
			name:           "Local firewall",
			port:           Port{Port: 8080, Firewall: "local"},
			expectedMode:   "local",
			expectedManage: true,
		},
		{
			name:           "Full firewall",
			port:           Port{Port: 8080, Firewall: "full"},
			expectedMode:   "full",
			expectedManage: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.port.FirewallMode(); got != tt.expectedMode {
				t.Errorf("FirewallMode() = %v, want %v", got, tt.expectedMode)
			}
			if got := tt.port.ShouldManageFirewall(); got != tt.expectedManage {
				t.Errorf("ShouldManageFirewall() = %v, want %v", got, tt.expectedManage)
			}
		})
	}
}

func TestFirewallValidation(t *testing.T) {
	service := &ServiceState{}

	tests := []struct {
		name        string
		config      *Config
		expectError bool
	}{
		{
			name: "Valid firewall settings",
			config: &Config{
				CheckIntervalSeconds: 5,
				Instances: []Instance{
					{
						Name: "Test",
						Ports: []Port{
							{Port: 8080, Firewall: "local"},
							{Port: 8081, Firewall: "full"},
							{Port: 8082}, // no firewall setting
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "Invalid firewall setting",
			config: &Config{
				CheckIntervalSeconds: 5,
				Instances: []Instance{
					{
						Name: "Test",
						Ports: []Port{
							{Port: 8080, Firewall: "invalid"},
						},
					},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.validateConfiguration(tt.config)
			if (err != nil) != tt.expectError {
				t.Errorf("validateConfiguration() error = %v, expectError = %v", err, tt.expectError)
			}
		})
	}
}

func TestFirewallRuleName(t *testing.T) {
	tests := []struct {
		port     int
		instance string
		expected string
	}{
		{8080, "Ubuntu-Dev", "WSL2-Port-8080-4815"}, // Calculated hash
		{22, "Ubuntu-ML", "WSL2-Port-22-2341"},      // Different instance, different hash
		{8080, "Ubuntu-Dev", "WSL2-Port-8080-4815"}, // Same input, same output
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("Port%d-%s", tt.port, tt.instance), func(t *testing.T) {
			got := generateFirewallRuleName(tt.port, tt.instance)
			if got != tt.expected {
				t.Errorf("generateFirewallRuleName(%d, %s) = %s, want %s", tt.port, tt.instance, got, tt.expected)
			}
		})
	}
}

// Helper function for string contains check
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
