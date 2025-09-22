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
	"unicode/utf16"
)

// Configuration structures
type Port struct {
	Port         int    `json:"port"`
	InternalPort int    `json:"internal_port,omitempty"`
	Firewall     string `json:"firewall,omitempty"` // "local", "full", or empty (warn only)
	Comment      string `json:"comment,omitempty"`
}

// ExternalPortEffective returns the external (listen) port
func (p Port) ExternalPortEffective() int {
	return p.Port
}

// InternalPortEffective returns the internal (connect) port, defaulting to external port if not specified
func (p Port) InternalPortEffective() int {
	if p.InternalPort != 0 {
		return p.InternalPort
	}
	return p.ExternalPortEffective()
}

// FirewallMode returns the firewall configuration mode
func (p Port) FirewallMode() string {
	return p.Firewall
}

// ShouldManageFirewall returns true if automatic firewall management is requested
func (p Port) ShouldManageFirewall() bool {
	return p.Firewall == "local" || p.Firewall == "full"
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
	ExternalPort int // Listen port on Windows host
	InternalPort int // Target port in WSL instance
	TargetIP     string
	Instance     string
	Comment      string
	FirewallMode string // "local", "full", or empty
}

type ServiceState struct {
	config           *Config
	configFile       string
	runningInstances map[string]string   // instance name -> IP address
	currentMappings  map[int]PortMapping // port -> mapping info
}

// decodeCommandOutput converts Windows command output from UTF-16LE to UTF-8 if needed
func decodeCommandOutput(output []byte) (string, error) {
	if len(output) == 0 {
		return "", nil
	}

	// Handle UTF-16 encoded output from Windows commands
	var outputStr string
	if len(output) > 0 && len(output)%2 == 0 {
		// Check if this looks like UTF-16 (every other byte is null or BOM present)
		isUTF16 := false
		
		// Check for UTF-16LE BOM
		if len(output) >= 2 && output[0] == 0xFF && output[1] == 0xFE {
			isUTF16 = true
			output = output[2:] // Skip BOM
		} else {
			// Check for interleaved null bytes (UTF-16LE pattern)
			for i := 1; i < len(output) && i < 20; i += 2 {
				if output[i] == 0 {
					isUTF16 = true
					break
				}
			}
		}

		if isUTF16 {
			// Convert UTF-16LE to UTF-8
			u16s := make([]uint16, len(output)/2)
			for i := 0; i < len(u16s); i++ {
				u16s[i] = uint16(output[i*2]) | uint16(output[i*2+1])<<8
			}
			runes := utf16.Decode(u16s)
			outputStr = string(runes)
		} else {
			outputStr = string(output)
		}
	} else {
		outputStr = string(output)
	}

	return outputStr, nil
}

func main() {
	// Check command line arguments
	if len(os.Args) < 2 || len(os.Args) > 3 {
		fmt.Println("Usage: wsl2-port-forwarder.exe [--validate] <config-file.json>")
		fmt.Println("")
		fmt.Println("Options:")
		fmt.Println("  --validate    Validate configuration and firewall rules, then exit")
		fmt.Println("")
		fmt.Println("Examples:")
		fmt.Println("  wsl2-port-forwarder.exe wsl2-config.json")
		fmt.Println("  wsl2-port-forwarder.exe --validate wsl2-config.json")
		os.Exit(1)
	}

	var validateOnly bool
	var configFile string

	if len(os.Args) == 3 {
		if os.Args[1] != "--validate" {
			fmt.Printf("Unknown option: %s\n", os.Args[1])
			os.Exit(1)
		}
		validateOnly = true
		configFile = os.Args[2]
	} else {
		configFile = os.Args[1]
	}

	if validateOnly {
		os.Exit(validateConfiguration(configFile))
	}

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

// handleFirewallRule manages firewall rules for a port mapping
func (s *ServiceState) handleFirewallRule(mapping PortMapping) {
	if mapping.FirewallMode == "" {
		// No firewall management requested
		return
	}

	if mapping.FirewallMode != "local" && mapping.FirewallMode != "full" {
		log.Printf("Warning: Invalid firewall mode '%s' for port %d, skipping firewall rule", mapping.FirewallMode, mapping.ExternalPort)
		return
	}

	log.Printf("Creating firewall rule for port %d (mode: %s, instance: %s)", mapping.ExternalPort, mapping.FirewallMode, mapping.Instance)

	if err := addFirewallRule(mapping.ExternalPort, mapping.Instance, mapping.FirewallMode); err != nil {
		log.Printf("Warning: Failed to create firewall rule for port %d: %v", mapping.ExternalPort, err)
		fmt.Printf("    ⚠️  Firewall rule creation failed: %v\n", err)
		fmt.Printf("    💡 Manual command: netsh advfirewall firewall add rule name=\"WSL2 Port %d\" dir=in action=allow protocol=TCP localport=%d remoteip=%s\n",
			mapping.ExternalPort, mapping.ExternalPort,
			map[string]string{"local": "LocalSubnet", "full": "any"}[mapping.FirewallMode])
	} else {
		log.Printf("Successfully created firewall rule for port %d", mapping.ExternalPort)
		fmt.Printf("    🔥 Firewall rule created: %s access to port %d\n",
			map[string]string{"local": "local network", "full": "any address"}[mapping.FirewallMode],
			mapping.ExternalPort)
	}
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

// validateConfiguration validates config file and optionally checks firewall rules
func validateConfiguration(configFile string) int {
	fmt.Println("WSL2 Port Forwarder - Configuration Validation")
	fmt.Println("=============================================")
	fmt.Printf("Config file: %s\n\n", configFile)

	exitCode := 0 // 0=success, 1=error, 2=warnings

	// Check if configuration file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		fmt.Printf("❌ Configuration file does not exist: %s\n", configFile)
		return 1
	}

	// Load and parse configuration
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		fmt.Printf("❌ Failed to read config file: %v\n", err)
		return 1
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		fmt.Printf("❌ Failed to parse JSON config: %v\n", err)
		return 1
	}

	// Validate configuration structure
	service := &ServiceState{}
	if err := service.validateConfiguration(&config); err != nil {
		fmt.Printf("❌ Configuration validation failed: %v\n", err)
		return 1
	}

	fmt.Printf("✅ Configuration syntax and structure: Valid\n")
	fmt.Printf("✅ Check interval: %d seconds\n", config.CheckIntervalSeconds)
	fmt.Printf("✅ Configured instances: %d\n\n", len(config.Instances))

	// Check for potential external port conflicts
	portToInstances := make(map[int][]string)
	for _, instance := range config.Instances {
		for _, port := range instance.Ports {
			externalPort := port.ExternalPortEffective()
			portToInstances[externalPort] = append(portToInstances[externalPort], instance.Name)
		}
	}

	conflictsFound := false
	for port, instances := range portToInstances {
		if len(instances) > 1 {
			if !conflictsFound {
				fmt.Println("⚠️  Potential external port conflicts (if instances run simultaneously):")
				conflictsFound = true
				exitCode = 2 // warnings
			}
			fmt.Printf("  Port %d: %s\n", port, strings.Join(instances, ", "))
			fmt.Printf("    → First instance (%s) will win, others ignored at runtime\n", instances[0])
		}
	}

	if conflictsFound {
		fmt.Println("\nℹ️  Note: Port conflicts are allowed if instances don't run simultaneously.")
		fmt.Println("    Examples: dev/staging/prod environments, or seasonal services.")
	} else {
		fmt.Println("✅ No external port conflicts detected")
	}

	// Validate Windows Firewall rules
	fmt.Println("\nℹ️  Checking Windows Firewall rules...")
	firewallExitCode := checkFirewallRules(&config)
	if firewallExitCode > exitCode {
		exitCode = firewallExitCode
	}

	// Summary
	fmt.Println("\n" + strings.Repeat("=", 50))
	switch exitCode {
	case 0:
		fmt.Println("✅ Configuration is valid and ready for use")
	case 1:
		fmt.Println("❌ Configuration has errors that must be fixed")
	case 2:
		fmt.Println("⚠️  Configuration is valid but has warnings")
	}

	return exitCode
}

// checkFirewallRules validates that Windows Firewall allows the configured ports
func checkFirewallRules(config *Config) int {
	exitCode := 0

	// Collect all unique external ports and their firewall settings
	ports := make(map[int]bool)
	firewallRules := make(map[int]string) // port -> firewall mode
	for _, instance := range config.Instances {
		for _, port := range instance.Ports {
			externalPort := port.ExternalPortEffective()
			ports[externalPort] = true
			if port.ShouldManageFirewall() {
				firewallRules[externalPort] = port.FirewallMode()
			}
		}
	}

	if len(ports) == 0 {
		fmt.Println("✅ No ports to check")
		return 0
	}

	// Check Windows Firewall rules using netsh
	cmd := exec.Command("netsh", "advfirewall", "firewall", "show", "rule", "name=all", "dir=in", "protocol=tcp")
	output, err := cmd.Output()
	if err != nil {
		fmt.Printf("⚠️  Unable to check firewall rules: %v\n", err)
		fmt.Println("    Please verify firewall rules manually")
		return 2
	}

	// Decode UTF-16 output from netsh
	outputStr, err := decodeCommandOutput(output)
	if err != nil {
		fmt.Printf("⚠️  Unable to decode firewall rules output: %v\n", err)
		fmt.Println("    Please verify firewall rules manually")
		return 2
	}

	// Parse firewall rules to find which TCP ports are allowed
	allowedPorts := make(map[int]bool)
	lines := strings.Split(outputStr, "\n")
	var currentRule string
	var isEnabled bool

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Look for rule name
		if strings.HasPrefix(line, "Rule Name:") {
			currentRule = strings.TrimPrefix(line, "Rule Name:")
			currentRule = strings.TrimSpace(currentRule)
			isEnabled = false
		}

		// Check if rule is enabled
		if strings.HasPrefix(line, "Enabled:") && strings.Contains(line, "Yes") {
			isEnabled = true
		}

		// Look for local ports
		if strings.HasPrefix(line, "LocalPort:") && isEnabled {
			portStr := strings.TrimPrefix(line, "LocalPort:")
			portStr = strings.TrimSpace(portStr)

			// Handle "Any" or specific ports
			if portStr == "Any" {
				// All ports are allowed by this rule
				for port := range ports {
					allowedPorts[port] = true
				}
			} else {
				// Parse specific ports (could be ranges or single ports)
				portParts := strings.Split(portStr, ",")
				for _, part := range portParts {
					part = strings.TrimSpace(part)
					if strings.Contains(part, "-") {
						// Port range
						rangeParts := strings.Split(part, "-")
						if len(rangeParts) == 2 {
							start, err1 := strconv.Atoi(strings.TrimSpace(rangeParts[0]))
							end, err2 := strconv.Atoi(strings.TrimSpace(rangeParts[1]))
							if err1 == nil && err2 == nil {
								for p := start; p <= end; p++ {
									if ports[p] {
										allowedPorts[p] = true
									}
								}
							}
						}
					} else {
						// Single port
						if port, err := strconv.Atoi(part); err == nil {
							if ports[port] {
								allowedPorts[port] = true
							}
						}
					}
				}
			}
		}
	}

	// Check which ports need firewall rules
	blockedPorts := make([]int, 0)
	for port := range ports {
		if !allowedPorts[port] {
			blockedPorts = append(blockedPorts, port)
		}
	}

	if len(blockedPorts) == 0 {
		fmt.Println("✅ All configured ports are allowed by Windows Firewall")
	} else {
		fmt.Printf("⚠️  %d port(s) may be blocked by Windows Firewall:\n", len(blockedPorts))
		for _, port := range blockedPorts {
			if mode, hasAuto := firewallRules[port]; hasAuto {
				fmt.Printf("  - Port %d (TCP) - Will be automatically managed (%s mode)\n", port, mode)
			} else {
				fmt.Printf("  - Port %d (TCP) - Manual firewall rule needed\n", port)
			}
		}

		// Show what automatic rules would be created
		automaticRules := false
		for _, port := range blockedPorts {
			if mode, hasAuto := firewallRules[port]; hasAuto {
				if !automaticRules {
					fmt.Println("\n🎆 Automatic firewall rules that will be created:")
					automaticRules = true
				}
				remoteIP := map[string]string{"local": "LocalSubnet", "full": "any"}[mode]
				accessType := map[string]string{"local": "local network", "full": "any address"}[mode]
				fmt.Printf("  Port %d: %s access (%s)\n", port, accessType, remoteIP)
			}
		}

		// Show manual commands for ports without automatic management
		manualPorts := make([]int, 0)
		for _, port := range blockedPorts {
			if _, hasAuto := firewallRules[port]; !hasAuto {
				manualPorts = append(manualPorts, port)
			}
		}

		if len(manualPorts) > 0 {
			fmt.Println("\nℹ️  Manual commands for remaining ports:")
			for _, port := range manualPorts {
				fmt.Printf("  netsh advfirewall firewall add rule name=\"WSL2 Port %d\" dir=in action=allow protocol=TCP localport=%d\n", port, port)
			}
			fmt.Println("\n  Or use Windows Firewall GUI: Control Panel > System and Security > Windows Firewall > Advanced Settings")
		}

		if !isRunningAsAdmin() && len(firewallRules) > 0 {
			fmt.Println("\n⚠️  Note: Admin privileges required for automatic firewall rule creation")
			fmt.Println("    Run as Administrator for automatic firewall management")
		}

		exitCode = 2
	}

	return exitCode
}

// isRunningAsAdmin checks if the current process has admin privileges
func isRunningAsAdmin() bool {
	// Try to create a firewall rule in test mode
	cmd := exec.Command("netsh", "advfirewall", "firewall", "show", "rule", "name=all")
	err := cmd.Run()
	return err == nil // If we can run netsh advfirewall commands, we likely have admin rights
}

// generateFirewallRuleName creates a unique firewall rule name
func generateFirewallRuleName(port int, instance string) string {
	// Create a short hash from instance name for uniqueness
	hash := 0
	for _, char := range instance {
		hash = hash*31 + int(char)
	}
	if hash < 0 {
		hash = -hash
	}
	return fmt.Sprintf("WSL2-Port-%d-%d", port, hash%10000)
}

// addFirewallRule creates a Windows Firewall rule for the specified port
func addFirewallRule(port int, instance string, mode string) error {
	if !isRunningAsAdmin() {
		return fmt.Errorf("admin privileges required for firewall rule creation")
	}

	ruleName := generateFirewallRuleName(port, instance)

	// Check if rule already exists
	checkCmd := exec.Command("netsh", "advfirewall", "firewall", "show", "rule", fmt.Sprintf("name=%s", ruleName))
	if checkCmd.Run() == nil {
		// Rule already exists, no need to create
		return nil
	}

	// Determine remote IP setting based on mode
	var remoteIP string
	switch mode {
	case "local":
		remoteIP = "LocalSubnet"
	case "full":
		remoteIP = "any"
	default:
		return fmt.Errorf("invalid firewall mode: %s", mode)
	}

	// Create the firewall rule
	cmd := exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
		fmt.Sprintf("name=%s", ruleName),
		"dir=in",
		"action=allow",
		"protocol=TCP",
		fmt.Sprintf("localport=%d", port),
		fmt.Sprintf("remoteip=%s", remoteIP),
		fmt.Sprintf("description=WSL2 port forwarding for %s", instance))

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create firewall rule: %v", err)
	}

	return nil
}

// removeFirewallRule removes a Windows Firewall rule
func removeFirewallRule(port int, instance string) error {
	if !isRunningAsAdmin() {
		return fmt.Errorf("admin privileges required for firewall rule removal")
	}

	ruleName := generateFirewallRuleName(port, instance)

	cmd := exec.Command("netsh", "advfirewall", "firewall", "delete", "rule", fmt.Sprintf("name=%s", ruleName))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to remove firewall rule: %v", err)
	}

	return nil
}

func (s *ServiceState) validateConfiguration(config *Config) error {
	// Validate check interval
	if config.CheckIntervalSeconds < 1 || config.CheckIntervalSeconds > 3600 {
		return fmt.Errorf("check_interval_seconds must be between 1 and 3600")
	}

	// Validate instances and ports
	for _, instance := range config.Instances {
		if instance.Name == "" {
			return fmt.Errorf("instance name cannot be empty")
		}

		for _, port := range instance.Ports {
			// Validate external port (required)
			if port.Port < 1 || port.Port > 65535 {
				return fmt.Errorf("invalid external port number %d in instance %s", port.Port, instance.Name)
			}

			// Validate internal port (optional, defaults to external port)
			if port.InternalPort != 0 && (port.InternalPort < 1 || port.InternalPort > 65535) {
				return fmt.Errorf("invalid internal port number %d in instance %s", port.InternalPort, instance.Name)
			}

			// Validate firewall field (optional)
			if port.Firewall != "" && port.Firewall != "local" && port.Firewall != "full" {
				return fmt.Errorf("invalid firewall setting '%s' for port %d in instance %s (must be 'local', 'full', or omitted)", port.Firewall, port.Port, instance.Name)
			}

			// Note: Duplicate external ports are allowed - instances may not run simultaneously
			// Runtime conflict resolution will handle cases where multiple instances with
			// the same external port are running at the same time
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

	// Decode UTF-16 output from WSL
	outputStr, err := decodeCommandOutput(output)
	if err != nil {
		return nil, fmt.Errorf("failed to decode WSL output: %v", err)
	}

	// Split by Windows line endings first, then Unix line endings as fallback
	var lines []string
	if strings.Contains(outputStr, "\r\n") {
		lines = strings.Split(strings.TrimSpace(outputStr), "\r\n")
	} else {
		lines = strings.Split(strings.TrimSpace(outputStr), "\n")
	}

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

	// Decode UTF-16 output from netsh
	outputStr, err := decodeCommandOutput(output)
	if err != nil {
		return nil, fmt.Errorf("failed to decode netsh output: %v", err)
	}

	mappings := make(map[int]PortMapping)
	lines := strings.Split(outputStr, "\n")

	// Parse netsh output - format varies by Windows version
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Look for lines containing port mappings
		// Format: "0.0.0.0         22          10.10.185.157   22"
		// Fields: [listenaddress, listenport, connectaddress, connectport]
		fields := strings.Fields(line)
		if len(fields) >= 4 {
			listenPort, err := strconv.Atoi(fields[1])
			if err != nil {
				continue
			}

			connectIP := fields[2]
			connectPort, err := strconv.Atoi(fields[3])
			if err != nil {
				continue
			}

			mappings[listenPort] = PortMapping{
				ExternalPort: listenPort,
				InternalPort: connectPort,
				TargetIP:     connectIP,
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

			externalPort := port.ExternalPortEffective()
			internalPort := port.InternalPortEffective()
			if externalPort == internalPort {
				fmt.Printf("    %d -> %s:%d%s\n", externalPort, ip, internalPort, portComment)
			} else {
				fmt.Printf("    %d -> %s:%d%s (external:%d -> internal:%d)\n", externalPort, ip, internalPort, portComment, externalPort, internalPort)
			}
		}
	}

	fmt.Println()
}

func (s *ServiceState) reconcilePortForwarding(currentMappings map[int]PortMapping) {
	fmt.Println("Checking port forwarding sync...")

	changesMade := false

	// Build desired state with conflict resolution
	desiredMappings := make(map[int]PortMapping)
	conflictedPorts := make(map[int][]string) // track conflicts for logging

	// Process instances in config file order (deterministic)
	for _, instance := range s.config.Instances {
		ip, isRunning := s.runningInstances[instance.Name]
		if !isRunning {
			continue
		}

		for _, port := range instance.Ports {
			externalPort := port.ExternalPortEffective()
			internalPort := port.InternalPortEffective()

			// Check if this external port is already claimed
			if existing, exists := desiredMappings[externalPort]; exists {
				// Port conflict! Log warning and ignore this instance's port
				log.Printf("WARNING: Instance '%s' port %d conflicts with '%s', ignoring",
					instance.Name, externalPort, existing.Instance)
				fmt.Printf("  ⚠️  Port conflict: Instance '%s' port %d ignored (conflicts with '%s')\n",
					instance.Name, externalPort, existing.Instance)

				// Track conflict for summary
				if conflictedPorts[externalPort] == nil {
					conflictedPorts[externalPort] = []string{existing.Instance}
				}
				conflictedPorts[externalPort] = append(conflictedPorts[externalPort], instance.Name)
				continue
			}

			// No conflict, add mapping
			desiredMappings[externalPort] = PortMapping{
				ExternalPort: externalPort,
				InternalPort: internalPort,
				TargetIP:     ip,
				Instance:     instance.Name,
				Comment:      port.Comment,
				FirewallMode: port.FirewallMode(),
			}
		}
	}

	// Display conflict summary if any conflicts occurred
	if len(conflictedPorts) > 0 {
		fmt.Println("\n⚠️  External port conflicts detected:")
		for externalPort, instances := range conflictedPorts {
			fmt.Printf("  Port %d: %s (winner) vs %s (ignored)\n",
				externalPort, instances[0], strings.Join(instances[1:], ", "))
		}
		fmt.Println("  First instance in config file wins, others ignored at runtime.")
		fmt.Println()
	}

	// Check for updates needed
	for port, desired := range desiredMappings {
		current, exists := currentMappings[port]

		if !exists {
			// Add new mapping
			if desired.ExternalPort == desired.InternalPort {
				fmt.Printf("  Adding port %d: None -> %s:%d\n", desired.ExternalPort, desired.TargetIP, desired.InternalPort)
			} else {
				fmt.Printf("  Adding port %d -> %d: None -> %s:%d\n", desired.ExternalPort, desired.InternalPort, desired.TargetIP, desired.InternalPort)
			}
			if err := s.addPortMapping(desired.ExternalPort, desired.InternalPort, desired.TargetIP); err != nil {
				log.Printf("Error adding port mapping %d->%d: %v", desired.ExternalPort, desired.InternalPort, err)
			} else {
				fmt.Printf("    ✓ Port %d->%d now forwarded to %s:%d\n", desired.ExternalPort, desired.InternalPort, desired.TargetIP, desired.InternalPort)
				changesMade = true

				// Handle firewall rule if requested
				s.handleFirewallRule(desired)
			}
		} else if current.TargetIP != desired.TargetIP || current.InternalPort != desired.InternalPort {
			// Update existing mapping
			if desired.ExternalPort == desired.InternalPort {
				fmt.Printf("  Updating port %d: %s:%d -> %s:%d\n", desired.ExternalPort, current.TargetIP, current.InternalPort, desired.TargetIP, desired.InternalPort)
			} else {
				fmt.Printf("  Updating port %d->%d: %s:%d -> %s:%d\n", desired.ExternalPort, desired.InternalPort, current.TargetIP, current.InternalPort, desired.TargetIP, desired.InternalPort)
			}
			if err := s.updatePortMapping(desired.ExternalPort, desired.InternalPort, desired.TargetIP); err != nil {
				log.Printf("Error updating port mapping %d->%d: %v", desired.ExternalPort, desired.InternalPort, err)
			} else {
				fmt.Printf("    ✓ Port %d->%d now forwarded to %s:%d\n", desired.ExternalPort, desired.InternalPort, desired.TargetIP, desired.InternalPort)
				changesMade = true

				// Handle firewall rule if requested
				s.handleFirewallRule(desired)
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
					if configPort.ExternalPortEffective() == port {
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

func (s *ServiceState) addPortMapping(externalPort int, internalPort int, targetIP string) error {
	cmd := exec.Command("netsh", "interface", "portproxy", "add", "v4tov4",
		fmt.Sprintf("listenport=%d", externalPort),
		"listenaddress=0.0.0.0",
		fmt.Sprintf("connectport=%d", internalPort),
		fmt.Sprintf("connectaddress=%s", targetIP))

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("netsh add command failed: %v", err)
	}

	return nil
}

func (s *ServiceState) updatePortMapping(externalPort int, internalPort int, targetIP string) error {
	// Remove existing mapping first
	if err := s.removePortMapping(externalPort); err != nil {
		return fmt.Errorf("failed to remove existing mapping: %v", err)
	}

	// Add new mapping
	return s.addPortMapping(externalPort, internalPort, targetIP)
}

func (s *ServiceState) removePortMapping(port int) error {
	cmd := exec.Command("netsh", "interface", "portproxy", "delete", "v4tov4",
		fmt.Sprintf("listenport=%d", port))

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("netsh delete command failed: %v", err)
	}

	return nil
}
