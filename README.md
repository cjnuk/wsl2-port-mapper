# WSL2 Port Forwarder Service

**Automatically manages port forwarding for WSL2 instances with dynamic IP addresses**

## Overview

WSL2 instances in NAT mode receive dynamic internal IP addresses (172.x.x.x range) that change when Windows reboots, WSL2 instances restart, or the WSL2 subsystem shuts down. This service automatically maintains consistent external access to services running in WSL2 instances without manual intervention.

## Features

- ✅ **Configuration-Driven**: JSON configuration file defines instance and port mappings
- ✅ **State Reconciliation**: Periodically compares desired vs actual port forwarding state  
- ✅ **Automatic Correction**: Updates port forwarding rules when WSL2 IP addresses change
- ✅ **Windows Service**: Runs automatically on system startup with restart on failure
- ✅ **Live Configuration**: Reloads config file changes without service restart
- ✅ **Zero Dependencies**: Single executable with no external requirements

## Security and Privacy (IMPORTANT)

⚠️ **DO NOT commit your personal `wsl2-config.json` file!** It contains your private instance names and port mappings.

### Multiple Security Layers:
- **`.gitignore`**: Blocks ALL `.json` files except `wsl2-config.example.json`
- **Git hooks**: Pre-commit hooks prevent accidental JSON commits
- **Secret scanning**: GitHub Actions run Gitleaks to detect sensitive data
- **Local protection**: Files stored in `%LOCALAPPDATA%\wsl2-port-mapper\` are safe

### Safe Configuration:
1. Copy `wsl2-config.example.json` → `wsl2-config.json` (locally)
2. Edit with your actual WSL2 instance names and ports
3. The file is automatically ignored by Git
4. Alternatively, store it outside the repo entirely

## Quick Start

### Prerequisites

1. **Windows 11 with WSL2** installed and configured
2. **Administrator privileges** for service installation
3. **WSL2 in NAT mode** (see WSL Configuration section below)

### Installation

1. **Download/Copy** all files to a directory (e.g., `C:\WSL2Service\`)
2. **Configure** your WSL2 instances and ports in `wsl2-config.json`
3. **Run as Administrator**: `install-service.bat`
4. **Start using** your port-forwarded services immediately

### Basic Usage

```bash
# Check service status
check-service.bat

# View current port forwarding rules
netsh interface portproxy show v4tov4

# Restart service if needed
nssm restart WSL2PortForwarder
```

## WSL Configuration

For optimal compatibility, update your `~/.wslconfig` (Windows user home) to use NAT networking:

```ini
[wsl2]
networkingMode=nat
memory=8GB
processors=4

[experimental]  
autoMemoryReclaim=dropCache
sparseVhd=true
```

After updating, restart WSL2:
```bash
wsl --shutdown
# Wait 10 seconds, then start your instances
```

## Configuration File

### Example `wsl2-config.json`

```json
{
  "check_interval_seconds": 5,
  "instances": [
    {
      "name": "Ubuntu-AI",
      "comment": "AI/ML development instance",
      "ports": [
        {
          "port": 2201,
          "internal_port": 22,
          "comment": "SSH access (external 2201 -> internal 22)"
        },
        {
          "port": 8001,
          "comment": "FastAPI server (same port internally)"
        },
        {
          "port": 8888,
          "comment": "Jupyter notebook"
        }
      ]
    },
    {
      "name": "Ubuntu-GPU", 
      "comment": "CUDA development",
      "ports": [
        {
          "port": 2202,
          "internal_port": 22,
          "comment": "SSH access (external 2202 -> internal 22)"
        },
        {
          "port": 8002,
          "internal_port": 6006,
          "comment": "TensorBoard (external 8002 -> internal 6006)"
        }
      ]
    }
  ]
}
```

### Configuration Rules

- ✅ **check_interval_seconds**: 1-3600 seconds (how often to check for changes)
- ✅ **instance names**: Must match exact WSL2 distribution names (`wsl -l`)
- ✅ **port numbers**: 1-65535, no duplicate **external** ports across all instances
- ✅ **internal_port** (optional): Target port inside WSL instance; defaults to same as `port`
- ✅ **comments**: Optional for both instances and ports
- ✅ **live reload**: Changes take effect on next check cycle (no restart needed)

### External vs Internal Port Mapping

**NEW FEATURE**: You can now map different external and internal ports!

- **`port`**: External port on Windows host (required)
- **`internal_port`**: Target port inside WSL instance (optional, defaults to `port`)

**Key Benefits:**
- Multiple WSL instances can use standard internal ports (like SSH port 22)
- External ports remain unique for each instance
- Backward compatible - existing configs work unchanged

**Examples:**
```json
// SSH access to different instances, all using port 22 internally
{ "port": 2201, "internal_port": 22, "comment": "SSH to Ubuntu-AI" }
{ "port": 2202, "internal_port": 22, "comment": "SSH to Ubuntu-GPU" }
{ "port": 2203, "internal_port": 22, "comment": "SSH to Ubuntu-Web" }

// Web services using standard HTTP/HTTPS ports internally
{ "port": 8080, "internal_port": 80,  "comment": "HTTP server" }
{ "port": 8443, "internal_port": 443, "comment": "HTTPS server" }

// Same external and internal port (legacy behavior)
{ "port": 3000, "comment": "Node.js dev server" }
```

## Service Management

### Installation Scripts

| Script | Purpose |
|--------|---------|
| `install-service.bat` | Install and configure Windows service |
| `uninstall-service.bat` | Remove service and optionally logs |
| `check-service.bat` | Display service status and recent logs |

### Manual Commands

```bash
# Service control
nssm start WSL2PortForwarder
nssm stop WSL2PortForwarder  
nssm restart WSL2PortForwarder

# Windows service commands
sc query WSL2PortForwarder
sc start WSL2PortForwarder
sc stop WSL2PortForwarder
```

### Log Files

Located in `logs/` directory:
- **service-output.log**: Normal operation output
- **service-error.log**: Error messages and warnings

Logs rotate automatically at 1MB with older versions preserved.

## Troubleshooting

### Common Issues

**Service won't start:**
- Verify WSL2 is installed: `wsl --version`
- Check config file syntax: valid JSON format
- Ensure no port conflicts with existing applications
- Run `check-service.bat` to see recent errors

**Port forwarding not working:**
- Confirm WSL2 instances are running: `wsl -l --running`
- Verify instance names match exactly (case-sensitive)
- Check Windows Firewall isn't blocking ports
- Ensure services are listening on 0.0.0.0 (not just 127.0.0.1) inside WSL2

**Config changes not taking effect:**
- Wait for next check cycle (5 seconds by default)
- Verify JSON syntax is valid
- Check service logs for parsing errors

### Debug Steps

1. **Check WSL2 status**: `wsl -l -v`
2. **Verify network mode**: Check `~/.wslconfig` has `networkingMode=nat`
3. **Test WSL2 connectivity**: From WSL2, ping Windows host
4. **Check current forwarding**: `netsh interface portproxy show v4tov4`
5. **Review service logs**: `check-service.bat`

## Directory Structure

```
WSL2Service/
├── wsl2-port-forwarder.exe    # Main service executable
├── wsl2-config.json           # Configuration file
├── nssm.exe                   # Service manager utility
├── install-service.bat        # Installation script
├── uninstall-service.bat      # Removal script
├── check-service.bat          # Status checker
├── README.md                  # This documentation
└── logs/
    ├── service-output.log     # Service output
    └── service-error.log      # Service errors
```

## Technical Details

### How It Works

1. **Discovery**: Queries WSL2 for running instances using `wsl --list --running`
2. **IP Detection**: Gets current IP for each instance via `wsl -d <name> -- hostname -I` 
3. **State Comparison**: Compares desired config vs current `netsh` forwarding rules
4. **Reconciliation**: Adds/updates/removes port forwarding rules as needed
5. **Wait & Repeat**: Sleeps for configured interval and repeats

### Port Forwarding Commands

```bash
# Add rule
netsh interface portproxy add v4tov4 listenport=8080 listenaddress=0.0.0.0 connectport=8080 connectaddress=172.18.144.5

# Remove rule  
netsh interface portproxy delete v4tov4 listenport=8080

# List all rules
netsh interface portproxy show v4tov4
```

### Performance

- **Memory Usage**: < 10MB typical, < 50MB maximum
- **CPU Usage**: < 1% average, < 5% during checks
- **Network Impact**: None (only local command execution)
- **Scalability**: Supports up to 50 instances, 100 ports each

## Cross-Host Deployment

This service works on any Windows 11 system with WSL2. To deploy:

1. **Copy entire directory** to target Windows system
2. **Update `wsl2-config.json`** for target system's WSL2 instances
3. **Run `install-service.bat`** as Administrator
4. **Service runs automatically** on startup

No modifications to scripts or executables needed - everything uses relative paths.

## License & Support

Built according to the WSL2-Port-Forwarder-Specification.md requirements.

For issues:
1. Run `check-service.bat` to gather diagnostic info
2. Check logs in `logs/` directory
3. Verify WSL2 configuration and instance names
4. Test port accessibility from within WSL2 instances

## Version Info

- **Build**: Phase 2 Complete (Service Integration)
- **Go Version**: 1.25.1
- **Target**: Windows 11 AMD64 with WSL2
- **Dependencies**: None (single executable + NSSM)