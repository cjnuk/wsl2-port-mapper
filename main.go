package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Configuration structures
type Port struct {
	Port    int    `json:"port"`
	Comment string `json:"comment,omitempty"`
}

type Instance struct {
	Name    string `json:"name"`
	Comment string `json:"comment,omitempty"`
	Ports   []Port `json:"ports"`
}

type Config struct {
	CheckIntervalSeconds int        `json:"check_interval_seconds"`
	Instances            []Instance `json:"instances"`
}

// Runtime state structures
type PortMapping struct {
	Port     int
	TargetIP string
	Instance string
	Comment  string
}

type ServiceState struct {
	config           *Config
	configFile       string
	runningInstances map[string]string   // instance name -> IP address
	currentMappings  map[int]PortMapping // port -> mapping info
}

func main() {
	// Check command line arguments
	if len(os.Args) != 2 {
		fmt.Println("Usage: wsl2-port-forwarder.exe <config-file.json>")
		os.Exit(1)
	}

	configFile := os.Args[1]

	// Initialize service state
	service := &ServiceState{
		configFile:       configFile,
		runningInstances: make(map[string]string),
		currentMappings:  make(map[int]PortMapping),
	}

	// Setup graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("\nReceived shutdown signal. Exiting gracefully...")
		os.Exit(0)
	}()

	// Validate initial setup
	if err := service.validateSetup(); err != nil {
		log.Fatalf("Setup validation failed: %v", err)
	}

	// Load and validate initial configuration
	if err := service.loadConfiguration(); err != nil {
		log.Fatalf("Failed to load initial configuration: %v", err)
	}

	fmt.Println("WSL2 Port Forwarding Service")
	fmt.Println("============================")
	fmt.Printf("Config file: %s\n", configFile)
	fmt.Printf("Check interval: %d seconds\n", service.config.CheckIntervalSeconds)
	fmt.Printf("Configured instances: %d\n", len(service.config.Instances))
	fmt.Println()

	// Main service loop
	for {
		service.serviceLoop()
		fmt.Printf("Waiting %d seconds...\n\n", service.config.CheckIntervalSeconds)
		time.Sleep(time.Duration(service.config.CheckIntervalSeconds) * time.Second)
	}
}

func (s *ServiceState) validateSetup() error {
	// Check if configuration file exists
	if _, err := os.Stat(s.configFile); os.IsNotExist(err) {
		return fmt.Errorf("configuration file does not exist: %s", s.configFile)
	}

	// Check if wsl.exe is available
	if _, err := exec.LookPath("wsl"); err != nil {
		return fmt.Errorf("wsl.exe not found in PATH")
	}

	// Check if netsh.exe is available
	if _, err := exec.LookPath("netsh"); err != nil {
		return fmt.Errorf("netsh.exe not found in PATH")
	}

	return nil
}

func (s *ServiceState) loadConfiguration() error {
	// Read configuration file
	data, err := ioutil.ReadFile(s.configFile)
	if err != nil {
		return fmt.Errorf("failed to read config file: %v", err)
	}

	// Parse JSON
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse JSON config: %v", err)
	}

	// Validate configuration
	if err := s.validateConfiguration(&config); err != nil {
		return fmt.Errorf("configuration validation failed: %v", err)
	}

	s.config = &config
	return nil
}

func (s *ServiceState) validateConfiguration(config *Config) error {
	// Validate check interval
	if config.CheckIntervalSeconds < 1 || config.CheckIntervalSeconds > 3600 {
		return fmt.Errorf("check_interval_seconds must be between 1 and 3600")
	}

	// Track used ports to detect duplicates
	usedPorts := make(map[int]string)

	// Validate instances and ports
	for _, instance := range config.Instances {
		if instance.Name == "" {
			return fmt.Errorf("instance name cannot be empty")
		}

		for _, port := range instance.Ports {
			if port.Port < 1 || port.Port > 65535 {
				return fmt.Errorf("invalid port number %d in instance %s", port.Port, instance.Name)
			}

			if existingInstance, exists := usedPorts[port.Port]; exists {
				return fmt.Errorf("duplicate port %d found in instances %s and %s", port.Port, existingInstance, instance.Name)
			}

			usedPorts[port.Port] = instance.Name
		}
	}

	return nil
}

func (s *ServiceState) serviceLoop() {
	// Reload configuration (live reload support)
	if err := s.loadConfiguration(); err != nil {
		log.Printf("Warning: Failed to reload configuration: %v", err)
		fmt.Println("Using previous configuration...")
	}

	// Get current running WSL2 instances
	runningInstances, err := s.getRunningWSLInstances()
	if err != nil {
		log.Printf("Error getting running WSL instances: %v", err)
		return
	}

	// Get IP addresses for running instances that are in our config
	s.runningInstances = make(map[string]string)
	for _, instance := range s.config.Instances {
		if _, isRunning := runningInstances[instance.Name]; isRunning {
			ip, err := s.getWSLInstanceIP(instance.Name)
			if err != nil {
				log.Printf("Warning: Failed to get IP for instance %s: %v", instance.Name, err)
				continue
			}
			s.runningInstances[instance.Name] = ip
		}
	}

	// Get current port forwarding state
	currentMappings, err := s.getCurrentPortMappings()
	if err != nil {
		log.Printf("Error getting current port mappings: %v", err)
		return
	}

	// Display current state
	s.displayCurrentState()

	// Calculate and apply required changes
	s.reconcilePortForwarding(currentMappings)
}

func (s *ServiceState) getRunningWSLInstances() (map[string]bool, error) {
	cmd := exec.Command("wsl", "--list", "--running", "--quiet")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute wsl --list --running: %v", err)
	}

	instances := make(map[string]bool)
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			instances[line] = true
		}
	}

	return instances, nil
}

func (s *ServiceState) getWSLInstanceIP(instanceName string) (string, error) {
	cmd := exec.Command("wsl", "-d", instanceName, "--", "hostname", "-I")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get IP for %s: %v", instanceName, err)
	}

	ip := strings.TrimSpace(string(output))
	// Take first IP if multiple are returned
	if ips := strings.Fields(ip); len(ips) > 0 {
		ip = ips[0]
	}

	// Validate IP format
	ipRegex := regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}$`)
	if !ipRegex.MatchString(ip) {
		return "", fmt.Errorf("invalid IP address format: %s", ip)
	}

	return ip, nil
}

func (s *ServiceState) getCurrentPortMappings() (map[int]PortMapping, error) {
	cmd := exec.Command("netsh", "interface", "portproxy", "show", "v4tov4")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute netsh command: %v", err)
	}

	mappings := make(map[int]PortMapping)
	lines := strings.Split(string(output), "\n")

	// Parse netsh output - format varies by Windows version
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Look for lines containing port mappings
		// Format: "listenport   listenaddress   connectport   connectaddress"
		fields := strings.Fields(line)
		if len(fields) >= 4 {
			listenPort, err := strconv.Atoi(fields[0])
			if err != nil {
				continue
			}

			connectIP := fields[3]

			mappings[listenPort] = PortMapping{
				Port:     listenPort,
				TargetIP: connectIP,
			}
		}
	}

	return mappings, nil
}

func (s *ServiceState) displayCurrentState() {
	fmt.Println("=== Current Port Forwarding State ===")

	// Display running instances
	runningNames := make([]string, 0, len(s.runningInstances))
	for name := range s.runningInstances {
		runningNames = append(runningNames, name)
	}

	if len(runningNames) > 0 {
		fmt.Printf("Running WSL2 instances: %s\n", strings.Join(runningNames, ", "))
	} else {
		fmt.Println("No configured WSL2 instances currently running")
	}

	fmt.Println("Active port forwarding:")

	// Display port mappings by instance
	for _, instance := range s.config.Instances {
		ip, isRunning := s.runningInstances[instance.Name]
		if !isRunning {
			continue
		}

		comment := ""
		if instance.Comment != "" {
			comment = fmt.Sprintf(" (%s)", instance.Comment)
		}

		fmt.Printf("  %s:%s\n", instance.Name, comment)

		for _, port := range instance.Ports {
			portComment := ""
			if port.Comment != "" {
				portComment = fmt.Sprintf(" (%s)", port.Comment)
			}

			fmt.Printf("    %d -> %s:%d%s\n", port.Port, ip, port.Port, portComment)
		}
	}

	fmt.Println()
}

func (s *ServiceState) reconcilePortForwarding(currentMappings map[int]PortMapping) {
	fmt.Println("Checking port forwarding sync...")

	changesMade := false

	// Build desired state
	desiredMappings := make(map[int]PortMapping)
	for _, instance := range s.config.Instances {
		ip, isRunning := s.runningInstances[instance.Name]
		if !isRunning {
			continue
		}

		for _, port := range instance.Ports {
			desiredMappings[port.Port] = PortMapping{
				Port:     port.Port,
				TargetIP: ip,
				Instance: instance.Name,
				Comment:  port.Comment,
			}
		}
	}

	// Check for updates needed
	for port, desired := range desiredMappings {
		current, exists := currentMappings[port]

		if !exists {
			// Add new mapping
			fmt.Printf("  Adding port %d: None -> %s\n", port, desired.TargetIP)
			if err := s.addPortMapping(port, desired.TargetIP); err != nil {
				log.Printf("Error adding port mapping %d: %v", port, err)
			} else {
				fmt.Printf("    ✓ Port %d now forwarded to %s\n", port, desired.TargetIP)
				changesMade = true
			}
		} else if current.TargetIP != desired.TargetIP {
			// Update existing mapping
			fmt.Printf("  Updating port %d: %s -> %s\n", port, current.TargetIP, desired.TargetIP)
			if err := s.updatePortMapping(port, desired.TargetIP); err != nil {
				log.Printf("Error updating port mapping %d: %v", port, err)
			} else {
				fmt.Printf("    ✓ Port %d now forwarded to %s\n", port, desired.TargetIP)
				changesMade = true
			}
		}
	}

	// Check for mappings to remove
	for port, _ := range currentMappings {
		if _, needed := desiredMappings[port]; !needed {
			// Check if this port belongs to one of our managed instances
			belongsToUs := false
			for _, instance := range s.config.Instances {
				for _, configPort := range instance.Ports {
					if configPort.Port == port {
						belongsToUs = true
						break
					}
				}
				if belongsToUs {
					break
				}
			}

			if belongsToUs {
				fmt.Printf("  Removing port %d (instance no longer running)\n", port)
				if err := s.removePortMapping(port); err != nil {
					log.Printf("Error removing port mapping %d: %v", port, err)
				} else {
					fmt.Printf("    ✓ Port %d mapping removed\n", port)
					changesMade = true
				}
			}
		}
	}

	if !changesMade {
		fmt.Println("  All port mappings are in sync")
	}
}

func (s *ServiceState) addPortMapping(port int, targetIP string) error {
	cmd := exec.Command("netsh", "interface", "portproxy", "add", "v4tov4",
		fmt.Sprintf("listenport=%d", port),
		"listenaddress=0.0.0.0",
		fmt.Sprintf("connectport=%d", port),
		fmt.Sprintf("connectaddress=%s", targetIP))

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("netsh add command failed: %v", err)
	}

	return nil
}

func (s *ServiceState) updatePortMapping(port int, targetIP string) error {
	// Remove existing mapping first
	if err := s.removePortMapping(port); err != nil {
		return fmt.Errorf("failed to remove existing mapping: %v", err)
	}

	// Add new mapping
	return s.addPortMapping(port, targetIP)
}

func (s *ServiceState) removePortMapping(port int) error {
	cmd := exec.Command("netsh", "interface", "portproxy", "delete", "v4tov4",
		fmt.Sprintf("listenport=%d", port))

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("netsh delete command failed: %v", err)
	}

	return nil
}
