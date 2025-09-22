package main

import (
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

func TestValidationDuplicateExternalPorts(t *testing.T) {
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
					{Port: 8080, InternalPort: 8080}, // Duplicate external port should fail
				},
			},
		},
	}

	err := service.validateConfiguration(config)
	if err == nil {
		t.Error("Expected validation error for duplicate external ports, got nil")
	}
	if err != nil && !contains(err.Error(), "duplicate external port") {
		t.Errorf("Expected error about duplicate external port, got: %v", err)
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

// Helper function for string contains check
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
