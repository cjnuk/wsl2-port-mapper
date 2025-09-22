#!/usr/bin/env uv run
# /// script
# dependencies = []
# ///

import subprocess
import json
import re
from collections import defaultdict
from typing import Dict, List, Set, Tuple

def get_wsl_instances() -> List[str]:
    """Get list of all WSL instances"""
    result = subprocess.run(['wsl', '--list', '--verbose'], capture_output=True, shell=True)
    if result.returncode != 0:
        print(f"Failed to get WSL instances: {result.stderr}")
        return []
    
    # Decode UTF-16LE output
    try:
        output = result.stdout.decode('utf-16le')
    except UnicodeDecodeError:
        try:
            output = result.stdout.decode('utf-8')
        except UnicodeDecodeError:
            output = result.stdout.decode('utf-8', errors='ignore')
    
    lines = output.strip().split('\n')[1:]  # Skip header
    instances = []
    for line in lines:
        line = line.strip()
        if not line:
            continue
            
        # Parse the verbose format: * NAME STATE VERSION or   NAME STATE VERSION
        parts = line.split()
        if len(parts) >= 3:
            # Remove the * marker if present
            name = parts[0] if not parts[0] == '*' else parts[1]
            # Clean up the instance name
            name = name.strip()
            if name and name != 'NAME':  # Skip header
                instances.append(name)
    
    return instances

def scan_instance_ports(instance: str) -> Dict[str, any]:
    """Scan a single WSL instance for listening ports"""
    print(f"Scanning {instance}...")
    
    # Try to get listening ports
    result = subprocess.run([
        'wsl', '-d', instance, '-u', 'root', '--', 
        'ss', '-tlnp'
    ], capture_output=True, text=True, shell=True)
    
    instance_info = {
        'instance': instance,
        'accessible': False,
        'ports': [],
        'services': {},
        'error': None
    }
    
    if result.returncode != 0:
        instance_info['error'] = f"Failed to access (exit {result.returncode}): {result.stderr.strip()}"
        return instance_info
    
    instance_info['accessible'] = True
    
    # Parse ss output
    lines = result.stdout.strip().split('\n')[1:]  # Skip header
    port_services = defaultdict(list)
    
    for line in lines:
        if 'LISTEN' not in line:
            continue
            
        # Parse ss output: State Recv-Q Send-Q Local_Address:Port Peer_Address:Port Process
        parts = line.split()
        if len(parts) < 4:
            continue
            
        local_addr = parts[3]
        if ':' not in local_addr:
            continue
            
        # Extract port
        try:
            addr, port_str = local_addr.rsplit(':', 1)
            port = int(port_str)
        except (ValueError, IndexError):
            continue
        
        # Skip loopback-only services for external mapping
        if addr.startswith('127.0.0') or addr.startswith('::1'):
            continue
            
        # Identify service type
        service_type = identify_service(port, line)
        
        port_info = {
            'port': port,
            'protocol': 'tcp',
            'address': addr,
            'service_type': service_type,
            'raw_line': line.strip()
        }
        
        instance_info['ports'].append(port_info)
        port_services[port].append(service_type)
    
    # Deduplicate and summarize
    unique_ports = {}
    for port_info in instance_info['ports']:
        port = port_info['port']
        if port not in unique_ports:
            unique_ports[port] = port_info
        
    instance_info['ports'] = list(unique_ports.values())
    instance_info['services'] = {port: list(set(services)) for port, services in port_services.items()}
    
    return instance_info

def identify_service(port: int, line: str) -> str:
    """Identify service type based on port number and process info"""
    # Common port mappings
    common_ports = {
        22: 'SSH',
        80: 'HTTP',
        443: 'HTTPS',
        3306: 'MySQL',
        5432: 'PostgreSQL',
        6379: 'Redis',
        6432: 'PgBouncer',
        631: 'CUPS',
        53: 'DNS',
        3000: 'Node.js Dev',
        8000: 'HTTP Dev',
        8080: 'HTTP Alt',
        5000: 'Flask Dev',
        3001: 'React Dev',
        4000: 'GraphQL',
        5678: 'Debug',
        7080: 'Proxy/LB'
    }
    
    # Check common ports first
    if port in common_ports:
        return common_ports[port]
    
    # Check for process names in the line
    if 'sshd' in line.lower():
        return 'SSH'
    elif 'nginx' in line.lower() or 'httpd' in line.lower() or 'apache' in line.lower():
        return 'HTTP'
    elif 'mysql' in line.lower() or 'mariadb' in line.lower():
        return 'MySQL'
    elif 'postgres' in line.lower():
        return 'PostgreSQL'
    elif 'redis' in line.lower():
        return 'Redis'
    elif 'node' in line.lower():
        return 'Node.js'
    elif 'python' in line.lower():
        return 'Python'
    elif 'code' in line.lower():
        return 'VS Code'
    
    # Port range guessing
    if 2000 <= port <= 2299:
        return 'SSH Alt'
    elif 3000 <= port <= 3999:
        return 'Dev Server'
    elif 8000 <= port <= 8999:
        return 'HTTP Alt'
    elif 5000 <= port <= 5999:
        return 'Dev/API'
    
    return 'Unknown'

def assign_external_ports(all_instances: List[Dict]) -> Dict[str, List[Dict]]:
    """Assign external ports systematically"""
    port_assignments = {}
    used_external_ports = set()
    conflicts = defaultdict(list)
    
    # Standard port mappings
    ssh_external = 2201
    http_external = 8081
    https_external = 8443
    mysql_external = 3307
    postgres_external = 5433
    redis_external = 6380
    dev_external = 9001
    
    for instance_info in all_instances:
        if not instance_info['accessible']:
            continue
            
        instance = instance_info['instance']
        port_assignments[instance] = []
        
        for port_info in instance_info['ports']:
            internal_port = port_info['port']
            service_type = port_info['service_type']
            
            # Assign external port based on service type
            external_port = None
            
            if service_type == 'SSH' or internal_port == 22:
                external_port = ssh_external
                ssh_external += 1
            elif service_type in ['HTTP', 'HTTP Alt'] and internal_port == 80:
                external_port = http_external
                http_external += 1
            elif service_type == 'HTTPS' and internal_port == 443:
                external_port = https_external
                https_external += 1
            elif service_type == 'MySQL' and internal_port == 3306:
                external_port = mysql_external
                mysql_external += 1
            elif service_type == 'PostgreSQL' and internal_port == 5432:
                external_port = postgres_external
                postgres_external += 1
            elif service_type == 'Redis' and internal_port == 6379:
                external_port = redis_external
                redis_external += 1
            elif service_type in ['Dev Server', 'Node.js Dev', 'Flask Dev']:
                external_port = dev_external
                dev_external += 1
            else:
                # For other services, try to use a port in 9000+ range
                external_port = 9000 + (internal_port % 1000)
                while external_port in used_external_ports:
                    external_port += 1
            
            # Check for conflicts
            if external_port in used_external_ports:
                conflicts[external_port].append({
                    'instance': instance,
                    'internal_port': internal_port,
                    'service_type': service_type
                })
            else:
                used_external_ports.add(external_port)
            
            mapping = {
                'name': f"{instance} {service_type}",
                'distro': instance,
                'protocol': 'tcp',
                'port': external_port,
                'internal_port': internal_port,
                'firewall': 'local',
                'listen_address': '0.0.0.0',
                'service_type': service_type,
                'original_address': port_info['address']
            }
            
            port_assignments[instance].append(mapping)
    
    return port_assignments, dict(conflicts)

def main():
    print("Scanning all WSL instances for listening services...\n")
    
    # Get all instances
    instances = get_wsl_instances()
    print(f"Found {len(instances)} WSL instances: {', '.join(instances)}\n")
    
    # Scan each instance
    all_instances = []
    for instance in instances:
        instance_info = scan_instance_ports(instance)
        all_instances.append(instance_info)
        
        if instance_info['accessible']:
            print(f"✓ {instance}: {len(instance_info['ports'])} listening ports")
        else:
            print(f"✗ {instance}: {instance_info['error']}")
    
    print("\n" + "="*60)
    print("DETAILED RESULTS")
    print("="*60)
    
    # Show detailed results
    total_mappings = 0
    for instance_info in all_instances:
        instance = instance_info['instance']
        print(f"\n{instance}:")
        
        if not instance_info['accessible']:
            print(f"  ERROR: {instance_info['error']}")
            continue
            
        if not instance_info['ports']:
            print("  No external listening ports found")
            continue
            
        for port_info in sorted(instance_info['ports'], key=lambda x: x['port']):
            port = port_info['port']
            service = port_info['service_type']
            addr = port_info['address']
            print(f"  Port {port:4d}: {service:12s} ({addr})")
            total_mappings += 1
    
    # Generate configuration
    port_assignments, conflicts = assign_external_ports(all_instances)
    
    # Create final configuration with proper structure
    config = {
        'check_interval_seconds': 5,
        'instances': []
    }
    
    for instance, mappings in port_assignments.items():
        # Convert mappings to the proper format
        ports_for_instance = []
        for mapping in mappings:
            port_config = {
                'port': mapping['port'],
                'internal_port': mapping['internal_port'],
                'firewall': mapping['firewall'],
                'comment': f"{mapping['service_type']} service (external {mapping['port']} -> internal {mapping['internal_port']})"
            }
            # Only include internal_port if it's different from port
            if mapping['port'] == mapping['internal_port']:
                del port_config['internal_port']
                port_config['comment'] = f"{mapping['service_type']} service (same port internally)"
            
            ports_for_instance.append(port_config)
        
        instance_config = {
            'name': instance,
            'comment': f"Auto-discovered WSL2 instance with {len(ports_for_instance)} services",
            'ports': ports_for_instance
        }
        config['instances'].append(instance_config)
    
    # Save configuration
    config_file = 'wsl2-services-config.json'
    with open(config_file, 'w') as f:
        json.dump(config, f, indent=2)
    
    print(f"\n" + "="*60)
    print("SUMMARY")
    print("="*60)
    print(f"Total accessible instances: {sum(1 for x in all_instances if x['accessible'])}")
    print(f"Total instances in config: {len(config['instances'])}")
    print(f"Total port mappings: {sum(len(instance['ports']) for instance in config['instances'])}")
    print(f"Configuration saved to: {config_file}")
    
    if conflicts:
        print(f"\nPORT CONFLICTS ({len(conflicts)} external ports affected):")
        for ext_port, conflict_list in conflicts.items():
            print(f"  External port {ext_port}:")
            for conflict in conflict_list:
                print(f"    - {conflict['instance']} (internal {conflict['internal_port']}, {conflict['service_type']})")
        print("\nNote: First instance in config file will win conflicts at runtime")
    
    # Service type summary
    service_counts = defaultdict(int)
    for instance_info in all_instances:
        if instance_info['accessible']:
            for port_info in instance_info['ports']:
                service_counts[port_info['service_type']] += 1
    
    print(f"\nSERVICE TYPES FOUND:")
    for service_type, count in sorted(service_counts.items()):
        print(f"  {service_type:15s}: {count:2d} instances")
    
    print(f"\nNext steps:")
    print(f"1. Review {config_file}")
    print(f"2. Test with: ./wsl2-port-mapper.exe --validate {config_file}")
    print(f"3. Apply with: ./wsl2-port-mapper.exe {config_file}")

if __name__ == '__main__':
    main()