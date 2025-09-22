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

4. **Service Framework**
   - NSSM-based Windows service installation
   - Graceful shutdown handling (SIGINT/SIGTERM)
   - Configurable check intervals with live configuration reload

5. **Conflict Resolution Engine**
   - Runtime handling of duplicate external ports across instances
   - First-configured-wins policy for simultaneous conflicts
   - Warning logging and status display for conflicts

### Data Flow

1. **Configuration Load**: Parse and validate `wsl2-config.json`
2. **WSL Discovery**: Query running WSL2 instances and their current IP addresses  
3. **State Reconciliation**: Compare desired port mappings vs current `netsh` state
4. **Windows Integration**: Apply changes via `netsh portproxy` and firewall commands
5. **Monitoring Loop**: Wait for configured interval and repeat

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
# Clean up: Remove-NetFirewallRule -DisplayName "WSL2-Port-8080-*"
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