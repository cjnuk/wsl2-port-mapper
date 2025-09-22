# 🛡️ Security Best Practices Guide

## 🚨 **Before You Deploy - Security Checklist**

### 1. **Never Commit Personal Information**

#### ⚠️ **High Risk Items to Avoid**
```bash
# NEVER commit files containing:
❌ Your actual WSL2 instance names
❌ Personal usernames or paths  
❌ Internal IP addresses or hostnames
❌ Service passwords or tokens
❌ SSH keys or certificates
❌ Environment variables with secrets
```

#### ✅ **Protected by .gitignore**
The repository automatically excludes:
- `*.json` (except wsl2-config.example.json)
- `logs/` directory and `*.log` files
- Personal discovery files (`*-discovered.json`, `*-complete.json`)
- Environment files (`.env`, `*.pem`, `*.key`)

#### 🔍 **Verify Before Pushing**
```bash
# Check what will be committed
git status
git ls-files | grep -E "(config|json)" 

# Should only show:
wsl2-config.example.json  ✅ Safe

# Should NOT show your personal configs
```

### 2. **Customize Safely for Your Environment**

#### Step 1: Copy Example Configuration
```bash
# Start with the safe template
cp wsl2-config.example.json wsl2-config.json

# Edit with YOUR instance names and ports
# This file is gitignored - safe to customize
```

#### Step 2: Update Instance Names
```json
{
  "instances": [
    {
      "name": "YOUR-ACTUAL-INSTANCE-NAME",
      "comment": "Replace with your real WSL2 instance names",
      "ports": [
        {"port": 22, "comment": "SSH access"},
        {"port": 8080, "comment": "Your application port"}
      ]
    }
  ]
}
```

#### Step 3: Discover Your Ports
```bash
# Find your WSL2 instances
wsl --list --verbose

# Check what's running in each instance  
wsl -u root -d YOUR-INSTANCE -- ss -tlnp
```

## 🔒 **Network Security Considerations**

### Port Exposure Assessment

#### **Localhost Only** (Default - Secure)
```bash
# Services bound to localhost/127.0.0.1 are NOT exposed externally
127.0.0.1:3306  # MySQL - localhost only ✅ Secure
```

#### **All Interfaces** (Potentially Exposed)  
```bash
# Services bound to 0.0.0.0 will be accessible from Windows network
0.0.0.0:22     # SSH - accessible from LAN ⚠️ Consider firewall
0.0.0.0:80     # Web server - accessible from LAN ⚠️ Consider firewall
```

### Firewall Configuration

#### Windows Defender Firewall Rules
```powershell
# Allow only specific source networks (replace 192.168.1.0/24 with your network)
New-NetFirewallRule -DisplayName "WSL2-SSH" -Direction Inbound -Protocol TCP -LocalPort 22 -RemoteAddress 192.168.1.0/24 -Action Allow

# Block external access to databases
New-NetFirewallRule -DisplayName "Block-MySQL-External" -Direction Inbound -Protocol TCP -LocalPort 3306 -RemoteAddress Any -Action Block
```

#### WSL2 Internal Firewall (Ubuntu/Debian)
```bash
# Install ufw if not present
sudo apt install ufw

# Default deny incoming
sudo ufw default deny incoming
sudo ufw default allow outgoing

# Allow SSH only from local network  
sudo ufw allow from 192.168.1.0/24 to any port 22

# Allow web access from local network
sudo ufw allow from 192.168.1.0/24 to any port 80,443

# Enable firewall
sudo ufw enable
```

## 🔐 **Authentication & Access Control**

### SSH Security Hardening

#### 1. **Disable Password Authentication**
```bash
# Edit SSH config in each WSL2 instance
sudo nano /etc/ssh/sshd_config

# Add/modify these lines:
PasswordAuthentication no
PubkeyAuthentication yes
PermitRootLogin no
AllowUsers yourusername
```

#### 2. **Use SSH Keys Only**
```bash
# Generate SSH key pair on Windows
ssh-keygen -t ed25519 -C "your_email@example.com"

# Copy public key to WSL2 instances
ssh-copy-id -p 22 username@localhost
```

#### 3. **Change Default SSH Ports**
```bash
# Use non-standard ports to avoid automated attacks
# In /etc/ssh/sshd_config:
Port 2222  # Instead of 22

# Update your wsl2-config.json accordingly
{"port": 2222, "comment": "SSH on non-standard port"}
```

### Web Service Security

#### 1. **Enable HTTPS Only**
```json
// In your config, prioritize HTTPS
{"port": 443, "comment": "HTTPS (secure)"},
// Remove or comment out HTTP
// {"port": 80, "comment": "HTTP (insecure)"}
```

#### 2. **Basic Authentication**
```bash
# For development web servers, add basic auth
# Nginx example:
auth_basic "Restricted";
auth_basic_user_file /etc/nginx/.htpasswd;
```

## 🕵️ **Monitoring & Incident Response**

### Log Monitoring

#### 1. **Enable Detailed Logging**  
```json
{
  "check_interval_seconds": 30,  // More frequent checks
  // ... your instances
}
```

#### 2. **Monitor Service Logs**
```bash
# Check port forwarder service logs
Get-Content logs\service-output.log -Tail 50

# Monitor SSH access attempts  
wsl -d YOUR-INSTANCE -- sudo tail -f /var/log/auth.log
```

#### 3. **Network Connection Monitoring**
```bash
# Monitor active connections
netstat -an | findstr :22
netstat -an | findstr :80

# Check for suspicious connections
wsl -- sudo netstat -tulnp | grep ESTABLISHED
```

### Incident Response

#### If You Suspect Unauthorized Access:

1. **Immediate Actions**
```bash
# Stop port forwarding service
nssm stop WSL2PortForwarder

# Check current connections  
netstat -an | findstr ESTABLISHED

# Review recent SSH logins
wsl -- sudo last -n 20
```

2. **Investigation**
```bash
# Check failed login attempts
wsl -- sudo grep "Failed password" /var/log/auth.log

# Review port forwarding rules
netsh interface portproxy show v4tov4

# Check for new/modified files
wsl -- sudo find /home -mtime -1 -type f
```

3. **Recovery**
```bash
# Change SSH keys
ssh-keygen -t ed25519 -f ~/.ssh/id_ed25519_new
ssh-copy-id -i ~/.ssh/id_ed25519_new.pub user@localhost

# Update WSL2 instances
wsl -- sudo apt update && sudo apt upgrade

# Review and update firewall rules
sudo ufw status numbered
```

## 📋 **Security Maintenance Checklist**

### Weekly
- [ ] Review service logs for anomalies
- [ ] Check active network connections
- [ ] Verify firewall rules are active
- [ ] Update WSL2 instances: `wsl -- sudo apt update && sudo apt upgrade`

### Monthly  
- [ ] Rotate SSH keys
- [ ] Review user accounts in WSL2 instances
- [ ] Check for new CVEs affecting your services
- [ ] Test backup/recovery procedures

### Before Major Changes
- [ ] Backup WSL2 instances: `wsl --export INSTANCE backup.tar`
- [ ] Document current port configurations
- [ ] Test rollback procedures
- [ ] Verify .gitignore still protects personal data

## 🆘 **Emergency Contacts & Resources**

### If You Accidentally Commit Sensitive Data
1. **Remove from Git History**
```bash
# Remove sensitive file from history
git filter-branch --force --index-filter 'git rm --cached --ignore-unmatch SENSITIVE-FILE' --prune-empty --tag-name-filter cat -- --all

# Force push (dangerous - coordinate with team)
git push origin --force --all
```

2. **Rotate Compromised Credentials**
- Change SSH keys immediately
- Update database passwords
- Revoke API tokens
- Review access logs

### Security Resources
- **Windows Security**: https://docs.microsoft.com/en-us/windows/security/
- **WSL2 Security**: https://docs.microsoft.com/en-us/windows/wsl/wsl-config
- **SSH Hardening**: https://stribika.github.io/2015/01/04/secure-secure-shell.html

Remember: **Security is a process, not a product**. Regular monitoring and maintenance are essential for keeping your development environment secure.