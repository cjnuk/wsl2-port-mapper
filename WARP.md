# WARP.md

This file provides guidance to WARP (warp.dev) when working with code in this repository.

## Project Overview

WSL2 Port Mapper is a Go-based Windows service that automatically manages port forwarding for WSL2 instances with dynamic IP addresses. It solves the problem of changing WSL2 IPs by continuously monitoring running instances and maintaining accurate netsh portproxy rules with optional automatic firewall management.

## Common Development Commands

### Build and Test
```bash
# Build the Go application
go build -o wsl2-port-forwarder.exe main.go

# Run tests with coverage
go test -v ./...
go test -cover ./...

# Run specific test
go test -run TestPortExternalPortEffective

# Build for production (optimized)
go build -ldflags "-s -w" -o dist/wsl2-port-forwarder.exe main.go
```

### Code Quality
```bash
# Lint and vet code
go vet ./...
go fmt ./...

# Static analysis (if golangci-lint available)
golangci-lint run

# Check module dependencies
go mod tidy
go mod verify
```

### Configuration Validation
```bash
# Validate configuration file syntax and rules
.\wsl2-port-forwarder.exe --validate wsl2-config.json

# Test with example configuration
copy wsl2-config.example.json wsl2-config.json
.\wsl2-port-forwarder.exe --validate wsl2-config.json
```

### Service Management
```bash
# Install service (requires Administrator)
.\install-service.bat

# Check service status
.\check-service.bat
sc query WSL2PortForwarder

# Service control via NSSM
nssm start WSL2PortForwarder
nssm stop WSL2PortForwarder
nssm restart WSL2PortForwarder

# Remove service
.\uninstall-service.bat
```

### Git Operations (Non-Interactive)
```bash
# Check repository status
git --no-pager status --porcelain
git --no-pager log --oneline -n 5
git --no-pager diff --stat

# Branch operations
git --no-pager branch -a
git --no-pager show --stat HEAD
```

### AI Automation Helpers
```bash
# Multi-agent code review
ai-runner multi-review main.go --cache --agents claude,gemini
ai-runner multi-review WARP.md --cache --agents claude,gemini

# Generate tests for new functionality
ai-runner generate-tests main.go --cache --agents claude,gemini

# Get consensus on architectural decisions
ai-runner consensus TECHNICAL-ARCHITECTURE.md --cache --agents claude,gemini,codex

# Analyze pull requests
ai-runner analyze-pr --cache --agents claude,gemini
```

## Architecture Overview

### Core Components

1. **Configuration System** (`Config`, `Instance`, `Port` structs)
   - JSON-based configuration with live reload support
   - Validation for port ranges, firewall modes, and instance names
   - Support for external→internal port mapping (e.g., 2201→22 for SSH)

2. **WSL2 Interface Layer**
   - WSL instance discovery via `wsl --list --running`
   - IP address detection via `wsl -d <instance> -- hostname -I`
   - UTF-16 output decoding for Windows command compatibility

3. **Windows Network Integration**
   - Port proxy management via `netsh interface portproxy`
   - Automatic Windows Firewall rule creation (local/full modes)
   - Service state reconciliation loop

4. **Registry-Based Resource Tracking** (`RegistryManager`)
   - Windows Registry integration for persistent resource tracking
   - Automatic registration/deregistration of port proxies and firewall rules
   - Built-in audit and cleanup capabilities to prevent orphaned resources
   - Registry structure: `HKLM\SOFTWARE\WSL2PortMapper\{PortProxies|FirewallRules}`

5. **Service Framework**
   - NSSM-based Windows service installation
   - Graceful shutdown handling (SIGINT/SIGTERM)
   - Configurable check intervals with live configuration reload

6. **Conflict Resolution Engine**
   - Runtime handling of duplicate external ports across instances
   - First-configured-wins policy for simultaneous conflicts
   - Warning logging and status display for conflicts

### Data Flow

1. **Configuration Load**: Parse and validate `wsl2-config.json`
2. **Registry Initialization**: Initialize Windows Registry tracking for resources
3. **WSL Discovery**: Query running WSL2 instances and their current IP addresses  
4. **State Reconciliation**: Compare desired port mappings vs current `netsh` state
5. **Windows Integration**: Apply changes via `netsh portproxy` and firewall commands
6. **Registry Tracking**: Register/unregister resources in Windows Registry for cleanup
7. **Automatic Cleanup**: Remove orphaned registry entries and stale resources
8. **Monitoring Loop**: Wait for configured interval and repeat

### Key Entry Points

- **CLI Mode**: `.\wsl2-port-forwarder.exe wsl2-config.json` (development/testing)
- **Validation Mode**: `.\wsl2-port-forwarder.exe --validate wsl2-config.json`
- **Service Mode**: Installed via `install-service.bat`, managed by Windows Service Manager

## WSL2 and Windows Port Mapping Specifics

### How Port Mapping Works

WSL2 instances receive dynamic IP addresses that change on:
- Windows reboots
- WSL2 instance restarts  
- WSL2 subsystem shutdown (`wsl --shutdown`)

**Port Proxy vs Port Forwarding:**
- **Port Proxy** (`netsh portproxy`): Application-layer TCP proxy within Windows host
- **Port Forwarding**: Network-layer packet forwarding (router/firewall level)
- WSL2 uses **port proxy** - Windows acts as TCP proxy between host and WSL2 network namespaces

The service maintains `netsh interface portproxy` rules that proxy Windows host ports to current WSL2 instance IPs.

### Manual Port Mapping Commands

```bash
# Show all current port proxies
netsh interface portproxy show v4tov4

# Add a port mapping (requires Administrator)
netsh interface portproxy add v4tov4 listenaddress=0.0.0.0 listenport=8080 connectaddress=172.18.144.5 connectport=8080

# Remove a mapping
netsh interface portproxy delete v4tov4 listenport=8080

# Reset all port proxies (caution!)
netsh interface portproxy reset
```

### Registry-Based Resource Tracking

The WSL2 Port Mapper now uses Windows Registry for robust resource tracking and automatic cleanup:

**Registry Structure:**
- `HKLM\SOFTWARE\WSL2PortMapper\PortProxies` - Tracks netsh port proxy entries
- `HKLM\SOFTWARE\WSL2PortMapper\FirewallRules` - Tracks Windows Firewall rules

**Automatic Operations:**
- All port proxies and firewall rules are registered in the registry when created
- Automatic cleanup removes orphaned registry entries during service operation
- Registry audit integrated into `--validate` command

```bash
# Audit registry vs actual system state
.\wsl2-port-mapper.exe --validate wsl2-config.json

# Registry audit is included automatically and shows:
# - Orphaned registry entries (registry has it, system doesn't)
# - Unregistered resources (system has it, registry doesn't)
# - Overall registry health status
```

### Manual Resource Audit Commands

```powershell
# Show all WSL2 Port Mapper firewall rules
Get-NetFirewallRule | Where-Object DisplayName -like "*WSL2-Port*" | Select-Object DisplayName, Enabled, Direction

# Show detailed firewall rule information
Get-NetFirewallRule -DisplayName "WSL2-Port-8080-*" | Get-NetFirewallAddressFilter

# Check registry entries (requires Administrator)
Get-ChildItem "HKLM:\SOFTWARE\WSL2PortMapper\PortProxies"
Get-ChildItem "HKLM:\SOFTWARE\WSL2PortMapper\FirewallRules"
    foreach ($instance in $runningInstances) {
        try {
            $instanceIP = (wsl -d $instance -- hostname -I).Trim().Split()[0]
            if ($instanceIP -eq $targetIP) { $hasRunningInstance = $true; break }
        } catch { }
    }
    if (-not $hasRunningInstance) {
        Write-Host "Orphaned port proxy: Port $port -> $targetIP (no running WSL2 instance)" -ForegroundColor Red
    }
}

# Enhanced audit: Show WSL2 Port Mapper managed resources
$wsl2Rules = Get-NetFirewallRule | Where-Object DisplayName -like "*WSL2-Port*WSL2PM"
$allProxies = netsh interface portproxy show v4tov4

Write-Host "WSL2 Port Mapper Managed Resources:" -ForegroundColor Green
foreach ($rule in $wsl2Rules) {
    # Extract port from rule name: WSL2-Port-8080-1234-WSL2PM
    if ($rule.DisplayName -match 'WSL2-Port-(\d+)-\d+-WSL2PM') {
        $port = $matches[1]
        $hasProxy = $allProxies -match "\s+$port\s+"
        $status = if ($hasProxy) { "✅ Active" } else { "❌ Orphaned Firewall Rule" }
        Write-Host "  Port $port : $status" -ForegroundColor $(if ($hasProxy) { 'Green' } else { 'Yellow' })
    }
}

# Clean up all WSL2 firewall rules (emergency cleanup)
# Get-NetFirewallRule | Where-Object DisplayName -like "*WSL2-Port*" | Remove-NetFirewallRule
```

### Firewall Management

```bash
# Add firewall rule for specific port (local network only)
netsh advfirewall firewall add rule name="WSL2 Port 8080" dir=in action=allow protocol=TCP localport=8080 remoteip=LocalSubnet

# Add firewall rule (allow any remote IP)
netsh advfirewall firewall add rule name="WSL2 Port 8080" dir=in action=allow protocol=TCP localport=8080

# Remove firewall rule
netsh advfirewall firewall delete rule name="WSL2 Port 8080"
```

### WSL IP Detection

```bash
# Get IP of default WSL instance
wsl -- hostname -I

# Get IP of specific WSL distribution  
wsl -d Ubuntu-Dev -- hostname -I

# List all running WSL instances
wsl --list --running
```

### Remote Windows Host Access

For multi-host development environments:

```bash
# Access Windows host admin session (per environment rules)
ssh psdt05 "netsh interface portproxy show v4tov4"
ssh psdt05 "wsl -u root Ubuntu-Dev ip -4 addr show eth0"

# Avoid redundant powershell.exe calls
ssh psdt05 "wsl --list --running"  # Good
# ssh psdt05 "powershell.exe -Command 'wsl --list --running'"  # Avoid
```

### Common Issues and Troubleshooting

**Services not accessible from external hosts:**
- Ensure WSL services bind to `0.0.0.0:PORT` not `127.0.0.1:PORT`
- Check Windows Firewall allows inbound connections
- Verify port proxy rules are active: `netsh interface portproxy show v4tov4`

**Port conflicts:**
- Use configuration validation: `--validate` flag shows conflicts before runtime
- Multiple instances with same external port: first in config file wins
- Check for existing applications using target ports

**WSL2 networking issues:**
- Confirm WSL2 uses NAT mode in `~/.wslconfig`: `networkingMode=nat`
- **Hyper-V firewall conflict**: If you see "Hyper-V firewall is not supported" and fallback to VirtioProxy:
  - **VirtioProxy mode works perfectly** with WSL2 Port Mapper (uses 10.x.x.x IP range)
  - **Recommended**: Explicitly set `networkingMode=virtioProxy` in `[wsl2]` section (no experimental features needed)
  - Alternative: Enable Hyper-V: `Enable-WindowsOptionalFeature -Online -FeatureName Microsoft-Hyper-V -All` (requires reboot)
  - Alternative: Remove experimental features that require Hyper-V (`firewall=true`, `dnsTunneling=true`)
- Restart WSL2 after config changes: `wsl --shutdown`
- Verify instance IP with: `wsl -d <name> -- hostname -I`
- Check current networking mode: `wsl --status` or observe startup messages

## Development Workflows

### Local Development Setup

1. **Configure your instances**: Copy `wsl2-config.example.json` → `wsl2-config.json`
2. **Validate configuration**: `.\wsl2-port-forwarder.exe --validate wsl2-config.json`
3. **Test run**: `.\wsl2-port-forwarder.exe wsl2-config.json` (Ctrl+C to stop)
4. **Install as service**: Run `.\install-service.bat` as Administrator

### Testing New Features

```bash
# Build and test changes
go build -o wsl2-port-forwarder.exe main.go
go test -v ./...

# Test configuration validation
.\wsl2-port-forwarder.exe --validate wsl2-config.json

# Test service loop once (manual verification)
.\wsl2-port-forwarder.exe wsl2-config.json
# Observe output, then Ctrl+C

# Install updated service
.\uninstall-service.bat
go build -o wsl2-port-forwarder.exe main.go  
.\install-service.bat
```

### Testing with VirtioProxy Networking Mode

If your WSL2 falls back to VirtioProxy mode instead of NAT:

**VirtioProxy Advantages:**
- Uses your physical network's IP range (e.g., 10.x.x.x instead of 172.x.x.x)
- WSL2 instances may share the same IP as Windows host (direct network integration)
- Potentially better performance (no NAT translation overhead)
- Simpler firewall management (same IP space as host)

**Important**: Even with shared IP addresses, **port mapping is still required** because:
- WSL2 runs in isolated network namespace (separate network stack)
- Windows host cannot directly access WSL2 services despite shared IP
- `netsh portproxy` bridges the network namespace isolation
- This is different from VMs - WSL2 uses "shared IP, isolated network stack" architecture

```bash
# Test if port mapping still works
.\wsl2-port-forwarder.exe wsl2-config.json
# Check for IP detection issues or mapping failures

# Verify WSL instance IP manually (may match Windows host IP)
wsl -d <instance-name> -- hostname -I
ipconfig | findstr "IPv4 Address"

# Test port proxy creation manually
netsh interface portproxy add v4tov4 listenaddress=0.0.0.0 listenport=8080 connectaddress=<wsl-ip> connectport=8080
netsh interface portproxy show v4tov4

# Remove test mapping
netsh interface portproxy delete v4tov4 listenport=8080

# Test automatic firewall rule creation
echo '{"check_interval_seconds": 5, "instances": [{"name": "<instance>", "ports": [{"port": 8080, "firewall": "local"}]}]}' > test-config.json
.\wsl2-port-forwarder.exe test-config.json
# Watch for firewall rule creation messages

# Test CRITICAL BUGS: Port proxy and firewall cleanup
wsl --terminate <instance>  # Stop the WSL2 instance

# BUG 1 TEST: Port proxy cleanup fails
netsh interface portproxy show v4tov4  # Will show proxy still active (BUG!)

# BUG 2 TEST: Firewall cleanup never happens  
Get-NetFirewallRule | Where-Object DisplayName -like "*WSL2-Port-8080*"  # Will show rule still active (BUG!)

# Manual cleanup until fix is implemented:
Remove-NetFirewallRule -DisplayName "WSL2-Port-8080-*"
```

### Optimal .wslconfig for VirtioProxy Mode

If your system doesn't support NAT mode or you prefer VirtioProxy:

```ini
# Optimized WSL2 Configuration for VirtioProxy Mode
# No experimental features, explicit networking mode
[wsl2]
memory=90GB 
processors=30
guiApplications=false
gpuSupport=true
swap=64GB
localhostforwarding=true
nestedVirtualization=false
debugConsole=false
vmIdleTimeout=-1

# Explicit VirtioProxy networking mode
networkingMode=virtioProxy

[general]
instanceIdleTimeout=-1
```

### Python Automation Scripts

For repository automation, prefer UV Python scripts over shell:

```bash
# Use UV for Python dependency management
uv add requests pyyaml
uv run scan_wsl_services.py

# Create UV-based automation scripts
# #!/usr/bin/env uv run
# # /// script
# # dependencies = ["requests"]  
# # ///
```

### Non-Interactive Command Patterns

Always use non-interactive flags to avoid hanging terminal sessions:

```bash
# Git operations
git --no-pager status --porcelain
git --no-pager log --oneline -n 10
git --no-pager diff --stat

# Service status
sc query WSL2PortForwarder | findstr STATE

# WSL information  
wsl --list --running --quiet
```

## Cross-References to Documentation

- **[README.md](README.md)**: Complete user guide, installation, and configuration
- **[TECHNICAL-ARCHITECTURE.md](TECHNICAL-ARCHITECTURE.md)**: Detailed technical implementation
- **[WSL2-Port-Forwarder-Specification.md](WSL2-Port-Forwarder-Specification.md)**: Original requirements and design
- **[wsl2-config.example.json](wsl2-config.example.json)**: Sample configuration with all features
- **[SECURITY.md](SECURITY.md)**: Security considerations and guidelines

## Environment-Specific Notes

**Windows 11 Development Hosts:**
- Use `ssh psdt05` for remote Windows host access (admin privileges available)
- Access WSL instances via Windows session: `ssh psdt05 "wsl -u root <instance>"`
- Direct root access when available: `ssh vllmr` (preferred over indirect access)

**Multi-Instance Management:**
- Configure multiple WSL2 instances with unique external ports
- Use shared external ports for dev/staging environments that don't run simultaneously
- **Leverage automatic firewall management:**
  - `"firewall": "local"` → `RemoteAddress: LocalSubnet` (secure for internal services)
  - `"firewall": "full"` → `RemoteAddress: Any` (public internet access)
  - Creates rules like `WSL2-Port-8080-<hash>` with proper port-specific configuration
  - Eliminates manual firewall rule creation and management

**CI/CD Integration:**
- Service can be deployed to multiple Windows hosts via directory copy
- Configuration files are environment-specific (never commit personal configs)
- Use `--validate` flag in CI pipelines to catch configuration errors early

## Critical Security Bugs Discovered

### Bug 1: Port Proxy Cleanup Fails Silently

**Issue**: `removePortMapping()` in `main.go` line 985-994 is missing `listenaddress=0.0.0.0` parameter.

**Impact**: Port proxy mappings are **NOT actually removed** when instances shut down!

**Current Buggy Code:**
```go
cmd := exec.Command("netsh", "interface", "portproxy", "delete", "v4tov4",
    fmt.Sprintf("listenport=%d", port))  // ❌ Missing listenaddress
```

**Fixed Code:**
```go
cmd := exec.Command("netsh", "interface", "portproxy", "delete", "v4tov4",
    "listenaddress=0.0.0.0",  // ✅ Must include listenaddress
    fmt.Sprintf("listenport=%d", port))
```

### Bug 2: Firewall Rules Never Cleaned Up

**Issue**: The service has `removeFirewallRule()` function but doesn't call it during reconciliation.

**Impact**: When WSL2 instances stop or are removed from configuration:
- ❌ Port proxy mappings remain active (Bug 1)
- ❌ Firewall rules remain active (Bug 2)

**Combined Impact**: **Complete failure of security cleanup** - both networking and firewall holes remain open indefinitely.

## Registry-Based Tracking System (Windows-Centric Solution)

### Problem: Reliable Resource Identification and Cleanup

**Current Issues:**
- Port proxy cleanup fails silently (Bug 1)
- Firewall cleanup never happens (Bug 2)  
- No reliable way to identify which resources belong to WSL2 Port Mapper
- `netsh portproxy` has no custom naming/description support

### Proposed Solution: Windows Registry Tracking

**Registry Structure:**
```
HKLM\SOFTWARE\WSL2PortMapper\
├── PortProxies\
│   ├── Port_8080 = '{"external_port":8080,"internal_port":80,"target_ip":"10.10.185.157","instance":"n8n","created":"2025-09-22T13:15:00Z"}'
│   └── Port_3501 = '{"external_port":3501,"internal_port":3501,"target_ip":"10.10.185.157","instance":"n8n","created":"2025-09-22T13:15:00Z"}'
└── FirewallRules\
    ├── Rule_8080_n8n = '{"port":8080,"rule_name":"WSL2-Port-8080-7556","instance":"n8n","created":"2025-09-22T13:15:00Z"}'
    └── Rule_3501_n8n = '{"port":3501,"rule_name":"WSL2-Port-3501-7556","instance":"n8n","created":"2025-09-22T13:15:00Z"}'
```

**Implementation Approach:**
```go
// Register port proxy creation
func (s *ServiceState) addPortMapping(externalPort, internalPort int, targetIP string) error {
    // Create netsh port proxy
    if err := createNetshPortProxy(externalPort, internalPort, targetIP); err != nil {
        return err
    }
    // Register in registry
    return registerPortProxy(externalPort, internalPort, targetIP, instanceName)
}

// Register firewall rule creation  
func addFirewallRule(port int, instance, mode string) error {
    ruleName := generateFirewallRuleName(port, instance)
    // Create Windows firewall rule
    if err := createWindowsFirewallRule(port, ruleName, mode); err != nil {
        return err
    }
    // Register in registry
    return registerFirewallRule(port, ruleName, instance)
}
```

**Benefits:**
- **Perfect identification**: Registry provides definitive source of truth
- **Rich metadata**: Store instance names, timestamps, port mappings
- **Windows-native**: Uses built-in Windows infrastructure
- **Atomic operations**: Can ensure registry and actual state stay synchronized
- **Audit capabilities**: Easy to detect inconsistencies between registry and reality

### Registry Management Commands

**PowerShell Registry Manager Script:**
```powershell
# Show current registry tracking status
.\registry-manager.ps1 Status

# Audit consistency between registry and actual state
.\registry-manager.ps1 Audit

# Clean up all registry tracking (does NOT remove actual resources)
.\registry-manager.ps1 Cleanup

# Repair registry by detecting orphaned WSL2 resources
.\registry-manager.ps1 Repair
```

**Manual Registry Inspection:**
```powershell
# View all tracked port proxies
Get-ChildItem "HKLM:\SOFTWARE\WSL2PortMapper\PortProxies" | ForEach-Object {
    $data = Get-ItemProperty -Path $_.PSPath -Name $_.Name
    $entry = $data.$($_.Name) | ConvertFrom-Json
    Write-Host "Port $($entry.external_port) → $($entry.target_ip):$($entry.internal_port) [$($entry.instance)]"
}

# View all tracked firewall rules
Get-ChildItem "HKLM:\SOFTWARE\WSL2PortMapper\FirewallRules" | ForEach-Object {
    $data = Get-ItemProperty -Path $_.PSPath -Name $_.Name  
    $entry = $data.$($_.Name) | ConvertFrom-Json
    Write-Host "Rule '$($entry.rule_name)' for Port $($entry.port) [$($entry.instance)]"
}

# Emergency cleanup (removes all WSL2PM registry entries)
Remove-Item -Path "HKLM:\SOFTWARE\WSL2PortMapper" -Recurse -Force
```

**Proposed Fix**: Modify `reconcilePortForwarding()` in `main.go` line ~944-952:
```go
// Current code only removes port mapping:
if err := s.removePortMapping(port); err != nil {
    log.Printf("Error removing port mapping %d: %v", port, err)
} else {
    fmt.Printf("    ✓ Port %d mapping removed\n", port)
    changesMade = true
    
    // ADD THIS: Remove corresponding firewall rule
    for _, instance := range s.config.Instances {
        for _, configPort := range instance.Ports {
            if configPort.ExternalPortEffective() == port && configPort.ShouldManageFirewall() {
                if err := removeFirewallRule(port, instance.Name); err != nil {
                    log.Printf("Warning: Failed to remove firewall rule for port %d: %v", port, err)
                } else {
                    fmt.Printf("    🔥 Firewall rule removed for port %d\n", port)
                }
            }
        }
    }
}
```

**Testing**: Use `Get-NetFirewallRule | Where-Object DisplayName -like "*WSL2-Port*"` to verify cleanup.
