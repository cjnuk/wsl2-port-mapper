# WSL2 Port Forwarder Service - Complete Specification

## Project Overview

Create a Windows service that automatically manages port forwarding for WSL2 instances. The service monitors running WSL2 instances and maintains accurate port forwarding rules that map Windows host ports to WSL2 instance services (SSH, APIs, web servers, etc.).

## Problem Statement

WSL2 instances in NAT mode receive dynamic internal IP addresses (172.x.x.x range) that change when:
- Windows host reboots
- WSL2 instances restart individually  
- WSL2 subsystem shuts down and restarts (`wsl --shutdown`)
- WSL2 auto-shutdown occurs after idle periods

This makes it impossible to maintain consistent external access to services running in WSL2 instances without manual intervention.

## Solution Requirements

### Core Functionality
1. **Configuration-Driven**: Read instance and port mappings from a JSON configuration file
2. **State Reconciliation**: Periodically compare desired port forwarding state with actual state
3. **Automatic Correction**: Update port forwarding rules when WSL2 IP addresses change
4. **Service Integration**: Run as a Windows service with automatic startup and restart capabilities
5. **Live Configuration**: Reload configuration file on each check cycle (no service restart required)

### Technical Requirements

#### Language and Platform
- **Language**: Go (golang)
- **Target Platform**: Windows 11 with WSL2
- **Architecture**: x64
- **Dependencies**: None (single executable)

#### Configuration Format
- **File Format**: JSON
- **Location**: Same directory as executable
- **Validation**: Detect and prevent duplicate port assignments
- **Comments**: Support optional comments for instances and ports

#### Service Characteristics
- **Startup Type**: Automatic (starts with Windows)
- **User Context**: Local System or Administrator
- **Restart Policy**: Automatic restart on failure
- **Logging**: Output to files (stdout and stderr)

## Detailed Specifications

### 1. Configuration File Structure

**File Name**: `wsl2-config.json`

**Schema**:
```json
{
  "check_interval_seconds": 5,
  "instances": [
    {
      "name": "Ubuntu-AI",
      "comment": "AI/ML development instance with GPU access",
      "ports": [
        {
          "port": 2201,
          "comment": "SSH access"
        },
        {
          "port": 8001,
          "comment": "FastAPI server"
        },
        {
          "port": 8888,
          "comment": "Jupyter notebook"
        }
      ]
    },
    {
      "name": "Ubuntu-GPU", 
      "comment": "CUDA development and training",
      "ports": [
        {
          "port": 2202,
          "comment": "SSH access"
        },
        {
          "port": 8002,
          "comment": "TensorBoard web interface"
        }
      ]
    }
  ]
}
```

**Validation Rules**:
- `check_interval_seconds`: Must be positive integer (1-3600)
- `instances[].name`: Must be valid WSL2 distribution name
- `instances[].ports[].port`: Must be valid port number (1-65535)
- No duplicate port numbers across all instances
- Comments are optional for both instances and ports

### 2. Core Application Logic

#### State Detection Flow
1. **Read Configuration**: Parse and validate JSON configuration file
2. **Discover Running Instances**: Execute `wsl --list --running --quiet` to get active WSL2 instances
3. **Get Instance IPs**: For each running instance in config, execute `wsl -d <instance> -- hostname -I` to get current IP
4. **Read Current Port Mappings**: Execute `netsh interface portproxy show v4tov4` and parse output
5. **Calculate Desired State**: Build map of desired port forwarding rules based on running instances
6. **Apply Differences**: Update/add/remove port forwarding rules as needed
7. **Wait and Repeat**: Sleep for configured interval and repeat process

#### Port Forwarding Operations
- **Add Rule**: `netsh interface portproxy add v4tov4 listenport=<port> listenaddress=0.0.0.0 connectport=<port> connectaddress=<wsl_ip>`
- **Remove Rule**: `netsh interface portproxy delete v4tov4 listenport=<port>`
- **Update Rule**: Delete existing rule, then add new rule (netsh overwrites automatically)

#### Error Handling
- Invalid WSL2 instance names: Log warning, continue with other instances
- WSL2 command failures: Log error with context, retry on next cycle
- Network command failures: Log error, continue operation
- Configuration file errors: Log error, continue with last valid configuration
- Service should never crash due to external command failures

### 3. Logging and Output

#### Standard Output Format
Display organized status information on each check:
```
WSL2 Port Forwarding Service
============================
Config file: wsl2-config.json

=== Current Port Forwarding State ===
Running WSL2 instances: Ubuntu-AI, Ubuntu-GPU
Active port forwarding:
  Ubuntu-AI: (AI/ML development with GPU access)
    2201 -> 172.18.144.5:2201 (SSH access)
    8001 -> 172.18.144.5:8001 (FastAPI server)
  Ubuntu-GPU: (CUDA development and training)
    2202 -> 172.18.144.6:2202 (SSH access)

Checking port forwarding sync...
  All port mappings are in sync
Waiting 5 seconds...
```

#### Change Detection Output
```
Checking port forwarding sync...
  Updating port 2201: 172.18.144.5 -> 172.18.150.3
    ✓ Port 2201 now forwarded to 172.18.150.3
  Adding port 8888: None -> 172.18.144.5
    ✓ Port 8888 now forwarded to 172.18.144.5
  Removing port 2203 (instance no longer running)
    ✓ Port 2203 mapping removed
```

#### Error Logging
- Include timestamps for all log entries
- Detailed error messages with context
- Non-fatal errors should not stop service operation
- Fatal errors should log reason before service termination

### 4. Command Line Interface

#### Arguments
- **Required**: Configuration file path as first argument
- **Example**: `wsl2-port-forwarder.exe wsl2-config.json`

#### Validation on Startup
- Verify configuration file exists and is readable
- Validate JSON syntax and schema
- Check for duplicate port assignments
- Verify `wsl.exe` and `netsh.exe` are available in PATH
- Display configuration summary before starting monitoring loop

#### Graceful Shutdown
- Handle SIGINT/SIGTERM signals (Ctrl+C)
- Display shutdown message
- Exit cleanly (important for service management)

### 5. Go Implementation Requirements

#### Package Structure
```
main.go                 // Single file implementation preferred
go.mod                  // Go module definition
```

#### Required Go Standard Library Packages
- `encoding/json` - Configuration parsing
- `fmt` - Output formatting
- `io/ioutil` - File operations
- `log` - Error logging
- `os` - Command line arguments, signals
- `os/exec` - Execute WSL and netsh commands
- `regexp` - Parse command output and validate IPs
- `strconv` - String to number conversions
- `strings` - String manipulation
- `time` - Sleep intervals and timestamps

#### Key Go Structures
```go
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
    Instances           []Instance `json:"instances"`
}
```

#### Build Requirements
- **Go Version**: 1.19 or later
- **Build Target**: Windows AMD64
- **Build Command**: `go build -o wsl2-port-forwarder.exe main.go`
- **Output**: Single executable with no external dependencies

### 6. Windows Service Installation

#### Service Manager: NSSM (Non-Sucking Service Manager)
- **Download Source**: https://nssm.cc/download
- **Required File**: `nssm.exe` (place in same directory as application)
- **Service Name**: `WSL2PortForwarder`

#### Service Configuration
```batch
nssm install WSL2PortForwarder "C:\full\path\to\wsl2-port-forwarder.exe" "C:\full\path\to\wsl2-config.json"
nssm set WSL2PortForwarder DisplayName "WSL2 Port Forwarder"
nssm set WSL2PortForwarder Description "Automatically manages port forwarding for WSL2 instances"
nssm set WSL2PortForwarder Start SERVICE_AUTO_START
nssm set WSL2PortForwarder AppExit Default Restart
nssm set WSL2PortForwarder AppRestartDelay 5000
nssm set WSL2PortForwarder AppStdout "C:\path\to\logs\service-output.log"
nssm set WSL2PortForwarder AppStderr "C:\path\to\logs\service-error.log"
```

#### Directory Structure
```
C:\WSL2Service\
├── wsl2-port-forwarder.exe
├── wsl2-config.json
├── nssm.exe
├── install-service.bat
├── uninstall-service.bat
├── check-service.bat
└── logs\
    ├── service-output.log
    └── service-error.log
```

### 7. Installation Scripts

#### install-service.bat
Requirements:
- Check for Administrator privileges
- Verify all required files exist (exe, config, nssm)
- Create logs directory
- Install service with proper configuration
- Offer to start service immediately
- Display management instructions

#### uninstall-service.bat
Requirements:
- Stop service if running
- Remove service registration
- Optionally remove log files
- Confirm successful removal

#### check-service.bat
Requirements:
- Display service status
- Show current port forwarding rules
- Display recent log entries (last 10 lines)
- Show running WSL2 instances

### 8. Cross-Host Compatibility

#### Generic Implementation Requirements
- Use relative paths where possible
- No hardcoded paths or host-specific configurations
- Configuration file defines all host-specific settings
- Service installation scripts use current directory as base path
- All file operations use proper path joining

#### Deployment Package Contents
- `wsl2-port-forwarder.exe` (compiled Go application)
- `wsl2-config.json` (template/example configuration)
- `nssm.exe` (service manager)
- `install-service.bat` (installation script)
- `uninstall-service.bat` (removal script)
- `check-service.bat` (status checker)
- `README.md` (setup and usage instructions)

### 9. Testing Requirements

#### Manual Testing Scenarios
1. **Basic Operation**: Service correctly forwards ports for running instances
2. **IP Change Detection**: Service updates forwarding when WSL2 IPs change after restart
3. **Instance Lifecycle**: Service handles starting/stopping WSL2 instances correctly
4. **Configuration Updates**: Service picks up configuration file changes without restart
5. **Error Recovery**: Service continues operation after temporary command failures
6. **Windows Reboot**: Service starts automatically and establishes correct port forwarding

#### Validation Commands
```batch
# Check service status
sc query WSL2PortForwarder

# Verify port forwarding
netsh interface portproxy show v4tov4

# Check WSL2 instances
wsl --list --running

# Test port connectivity
telnet localhost <port>
```

### 10. Performance and Resource Requirements

#### Resource Usage Targets
- **Memory Usage**: < 10MB typical, < 50MB maximum
- **CPU Usage**: < 1% average, < 5% during check cycles
- **Disk I/O**: Minimal (only config file reads and log writes)
- **Network Impact**: None (only local command execution)

#### Scalability Limits
- **Maximum Instances**: 50 WSL2 instances
- **Maximum Ports per Instance**: 100 ports
- **Maximum Total Ports**: 1000 ports
- **Minimum Check Interval**: 1 second
- **Maximum Check Interval**: 1 hour (3600 seconds)

## Implementation Instructions for AI Agent

### Phase 1: Core Application Development
1. Create Go module and implement main application logic
2. Implement configuration file parsing and validation
3. Implement WSL2 instance detection and IP retrieval
4. Implement port forwarding state management
5. Add comprehensive error handling and logging
6. Test application manually with sample configuration

### Phase 2: Service Integration
1. Download NSSM from official source
2. Create service installation scripts
3. Create service management utilities
4. Test service installation and operation
5. Verify automatic startup and restart functionality

### Phase 3: Deployment Package
1. Create complete deployment package
2. Generate comprehensive README documentation
3. Test deployment on clean Windows system
4. Verify cross-host compatibility

### Phase 4: Validation
1. Test all specified scenarios
2. Verify resource usage requirements
3. Confirm reliable operation over extended periods
4. Validate configuration file live reload functionality

## Success Criteria

- ✅ Single executable runs as Windows service
- ✅ Automatically manages WSL2 port forwarding with zero manual intervention
- ✅ Configuration changes take effect without service restart
- ✅ Service survives Windows reboots, WSL2 restarts, and network changes
- ✅ Clear logging and status information for troubleshooting
- ✅ Easy deployment to multiple Windows WSL2 hosts
- ✅ Reliable operation with minimal resource usage

This specification provides complete requirements for building a production-ready WSL2 port forwarding service that solves the dynamic IP address problem in a robust, maintainable, and deployable manner.