package main

import (
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/windows/registry"
)

const (
	// Registry paths for tracking WSL2 Port Mapper resources
	registryBasePath    = "SOFTWARE\\WSL2PortMapper"
	portProxyPath       = registryBasePath + "\\PortProxies"
	firewallRulesPath   = registryBasePath + "\\FirewallRules"
)

// RegistryPortProxy represents a port proxy entry in the registry
type RegistryPortProxy struct {
	Key            string
	ListenPort     int
	ConnectAddress string
	ConnectPort    int
	Instance       string
	Timestamp      string
}

// RegistryFirewallRule represents a firewall rule entry in the registry
type RegistryFirewallRule struct {
	Key       string
	RuleName  string
	Port      string
	Instance  string
	Timestamp string
}

// RegistryManager handles all Windows Registry operations for tracking resources
type RegistryManager struct {
	baseKey         registry.Key
	portProxyKey    registry.Key
	firewallRuleKey registry.Key
}

// NewRegistryManager creates and initializes a new registry manager
func NewRegistryManager() (*RegistryManager, error) {
	rm := &RegistryManager{}
	
	if err := rm.initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize registry manager: %v", err)
	}
	
	return rm, nil
}

// initialize creates the registry structure if it doesn't exist
func (rm *RegistryManager) initialize() error {
	// Open or create the base registry key
	baseKey, _, err := registry.CreateKey(registry.LOCAL_MACHINE, registryBasePath, registry.ALL_ACCESS)
	if err != nil {
		return fmt.Errorf("failed to create base registry key: %v", err)
	}
	rm.baseKey = baseKey
	
	// Open or create the port proxy tracking key
	portProxyKey, _, err := registry.CreateKey(registry.LOCAL_MACHINE, portProxyPath, registry.ALL_ACCESS)
	if err != nil {
		baseKey.Close()
		return fmt.Errorf("failed to create port proxy registry key: %v", err)
	}
	rm.portProxyKey = portProxyKey
	
	// Open or create the firewall rules tracking key
	firewallRuleKey, _, err := registry.CreateKey(registry.LOCAL_MACHINE, firewallRulesPath, registry.ALL_ACCESS)
	if err != nil {
		baseKey.Close()
		portProxyKey.Close()
		return fmt.Errorf("failed to create firewall rules registry key: %v", err)
	}
	rm.firewallRuleKey = firewallRuleKey
	
	log.Printf("Registry manager initialized successfully")
	return nil
}

// Close releases all registry handles
func (rm *RegistryManager) Close() error {
	var errs []error
	
	if rm.firewallRuleKey != 0 {
		if err := rm.firewallRuleKey.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	
	if rm.portProxyKey != 0 {
		if err := rm.portProxyKey.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	
	if rm.baseKey != 0 {
		if err := rm.baseKey.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	
	if len(errs) > 0 {
		return fmt.Errorf("errors closing registry keys: %v", errs)
	}
	
	return nil
}

// RegisterPortProxy adds a port proxy entry to the registry
func (rm *RegistryManager) RegisterPortProxy(listenPort int, connectAddress string, connectPort int, instance string) error {
	key := fmt.Sprintf("proxy_%d_%s", listenPort, time.Now().Format("20060102_150405"))
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	
	// Create registry subkey for this port proxy
	proxyKey, _, err := registry.CreateKey(rm.portProxyKey, key, registry.ALL_ACCESS)
	if err != nil {
		return fmt.Errorf("failed to create port proxy registry entry: %v", err)
	}
	defer proxyKey.Close()
	
	// Set registry values
	if err := proxyKey.SetDWordValue("ListenPort", uint32(listenPort)); err != nil {
		return fmt.Errorf("failed to set ListenPort: %v", err)
	}
	
	if err := proxyKey.SetStringValue("ConnectAddress", connectAddress); err != nil {
		return fmt.Errorf("failed to set ConnectAddress: %v", err)
	}
	
	if err := proxyKey.SetDWordValue("ConnectPort", uint32(connectPort)); err != nil {
		return fmt.Errorf("failed to set ConnectPort: %v", err)
	}
	
	if err := proxyKey.SetStringValue("Instance", instance); err != nil {
		return fmt.Errorf("failed to set Instance: %v", err)
	}
	
	if err := proxyKey.SetStringValue("Timestamp", timestamp); err != nil {
		return fmt.Errorf("failed to set Timestamp: %v", err)
	}
	
	log.Printf("Registered port proxy in registry: %d -> %s:%d (%s)", listenPort, connectAddress, connectPort, instance)
	return nil
}

// UnregisterPortProxy removes port proxy entries from the registry
func (rm *RegistryManager) UnregisterPortProxy(listenPort int) error {
	// Find all registry entries for this port
	entries, err := rm.GetRegisteredPortProxies()
	if err != nil {
		return fmt.Errorf("failed to get registered port proxies: %v", err)
	}
	
	var deleted int
	for _, entry := range entries {
		if entry.ListenPort == listenPort {
			if err := registry.DeleteKey(rm.portProxyKey, entry.Key); err != nil {
				log.Printf("Warning: failed to delete port proxy registry entry %s: %v", entry.Key, err)
			} else {
				deleted++
				log.Printf("Unregistered port proxy from registry: %s", entry.Key)
			}
		}
	}
	
	if deleted == 0 {
		log.Printf("Warning: no registry entries found for port proxy %d", listenPort)
	}
	
	return nil
}

// RegisterFirewallRule adds a firewall rule entry to the registry
func (rm *RegistryManager) RegisterFirewallRule(ruleName string, port int, instance string) error {
	key := fmt.Sprintf("fw_%d_%s", port, time.Now().Format("20060102_150405"))
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	
	// Create registry subkey for this firewall rule
	ruleKey, _, err := registry.CreateKey(rm.firewallRuleKey, key, registry.ALL_ACCESS)
	if err != nil {
		return fmt.Errorf("failed to create firewall rule registry entry: %v", err)
	}
	defer ruleKey.Close()
	
	// Set registry values
	if err := ruleKey.SetStringValue("RuleName", ruleName); err != nil {
		return fmt.Errorf("failed to set RuleName: %v", err)
	}
	
	if err := ruleKey.SetDWordValue("Port", uint32(port)); err != nil {
		return fmt.Errorf("failed to set Port: %v", err)
	}
	
	if err := ruleKey.SetStringValue("Instance", instance); err != nil {
		return fmt.Errorf("failed to set Instance: %v", err)
	}
	
	if err := ruleKey.SetStringValue("Timestamp", timestamp); err != nil {
		return fmt.Errorf("failed to set Timestamp: %v", err)
	}
	
	log.Printf("Registered firewall rule in registry: %s (port %d, instance %s)", ruleName, port, instance)
	return nil
}

// UnregisterFirewallRule removes firewall rule entries from the registry by rule name
func (rm *RegistryManager) UnregisterFirewallRule(ruleName string) error {
	// Find all registry entries for this rule name
	entries, err := rm.GetRegisteredFirewallRules()
	if err != nil {
		return fmt.Errorf("failed to get registered firewall rules: %v", err)
	}
	
	var deleted int
	for _, entry := range entries {
		if entry.RuleName == ruleName {
			if err := registry.DeleteKey(rm.firewallRuleKey, entry.Key); err != nil {
				log.Printf("Warning: failed to delete firewall rule registry entry %s: %v", entry.Key, err)
			} else {
				deleted++
				log.Printf("Unregistered firewall rule from registry: %s", entry.Key)
			}
		}
	}
	
	if deleted == 0 {
		log.Printf("Warning: no registry entries found for firewall rule %s", ruleName)
	}
	
	return nil
}

// GetRegisteredPortProxies retrieves all registered port proxy entries
func (rm *RegistryManager) GetRegisteredPortProxies() ([]RegistryPortProxy, error) {
	entries := []RegistryPortProxy{}
	
	subkeys, err := rm.portProxyKey.ReadSubKeyNames(-1)
	if err != nil {
		return entries, fmt.Errorf("failed to read port proxy subkeys: %v", err)
	}
	
	for _, subkey := range subkeys {
		proxyKey, err := registry.OpenKey(rm.portProxyKey, subkey, registry.QUERY_VALUE)
		if err != nil {
			log.Printf("Warning: failed to open port proxy subkey %s: %v", subkey, err)
			continue
		}
		
		entry := RegistryPortProxy{Key: subkey}
		
		// Read values
		if listenPort, _, err := proxyKey.GetIntegerValue("ListenPort"); err == nil {
			entry.ListenPort = int(listenPort)
		}
		
		if connectAddress, _, err := proxyKey.GetStringValue("ConnectAddress"); err == nil {
			entry.ConnectAddress = connectAddress
		}
		
		if connectPort, _, err := proxyKey.GetIntegerValue("ConnectPort"); err == nil {
			entry.ConnectPort = int(connectPort)
		}
		
		if instance, _, err := proxyKey.GetStringValue("Instance"); err == nil {
			entry.Instance = instance
		}
		
		if timestamp, _, err := proxyKey.GetStringValue("Timestamp"); err == nil {
			entry.Timestamp = timestamp
		}
		
		entries = append(entries, entry)
		proxyKey.Close()
	}
	
	return entries, nil
}

// GetRegisteredFirewallRules retrieves all registered firewall rule entries
func (rm *RegistryManager) GetRegisteredFirewallRules() ([]RegistryFirewallRule, error) {
	entries := []RegistryFirewallRule{}
	
	subkeys, err := rm.firewallRuleKey.ReadSubKeyNames(-1)
	if err != nil {
		return entries, fmt.Errorf("failed to read firewall rule subkeys: %v", err)
	}
	
	for _, subkey := range subkeys {
		ruleKey, err := registry.OpenKey(rm.firewallRuleKey, subkey, registry.QUERY_VALUE)
		if err != nil {
			log.Printf("Warning: failed to open firewall rule subkey %s: %v", subkey, err)
			continue
		}
		
		entry := RegistryFirewallRule{Key: subkey}
		
		// Read values
		if ruleName, _, err := ruleKey.GetStringValue("RuleName"); err == nil {
			entry.RuleName = ruleName
		}
		
		if port, _, err := ruleKey.GetIntegerValue("Port"); err == nil {
			entry.Port = strconv.Itoa(int(port))
		}
		
		if instance, _, err := ruleKey.GetStringValue("Instance"); err == nil {
			entry.Instance = instance
		}
		
		if timestamp, _, err := ruleKey.GetStringValue("Timestamp"); err == nil {
			entry.Timestamp = timestamp
		}
		
		entries = append(entries, entry)
		ruleKey.Close()
	}
	
	return entries, nil
}

// AuditRegistryState compares registry entries with actual system state
func (rm *RegistryManager) AuditRegistryState() (bool, error) {
	fmt.Println("=== Auditing Registry vs Actual State ===")
	
	allGood := true
	
	// Audit port proxies
	fmt.Println("\n--- Port Proxy Audit ---")
	if err := rm.auditPortProxies(); err != nil {
		fmt.Printf("Error auditing port proxies: %v\n", err)
		allGood = false
	}
	
	// Audit firewall rules
	fmt.Println("\n--- Firewall Rules Audit ---")
	if err := rm.auditFirewallRules(); err != nil {
		fmt.Printf("Error auditing firewall rules: %v\n", err)
		allGood = false
	}
	
	if allGood {
		fmt.Println("\n✅ All registry entries match actual system state")
	} else {
		fmt.Println("\n⚠️  Registry inconsistencies detected")
	}
	
	return allGood, nil
}

// auditPortProxies checks port proxy registry vs actual netsh state
func (rm *RegistryManager) auditPortProxies() error {
	registered, err := rm.GetRegisteredPortProxies()
	if err != nil {
		return err
	}
	
	// Get actual port proxies from the system (reuse existing logic)
	service := &ServiceState{}
	actual, err := service.getCurrentPortMappings()
	if err != nil {
		return err
	}
	
	// Check for orphaned registry entries
	orphaned := 0
	for _, reg := range registered {
		found := false
		for _, act := range actual {
			if reg.ListenPort == act.ExternalPort &&
				reg.ConnectAddress == act.TargetIP &&
				reg.ConnectPort == act.InternalPort {
				found = true
				break
			}
		}
		if !found {
			fmt.Printf("  ORPHANED: Registry has %d -> %s:%d but not found in netsh\n",
				reg.ListenPort, reg.ConnectAddress, reg.ConnectPort)
			orphaned++
		}
	}
	
	// Check for unregistered actual proxies
	unregistered := 0
	for _, act := range actual {
		found := false
		for _, reg := range registered {
			if reg.ListenPort == act.ExternalPort &&
				reg.ConnectAddress == act.TargetIP &&
				reg.ConnectPort == act.InternalPort {
				found = true
				break
			}
		}
		if !found {
			fmt.Printf("  UNREGISTERED: netsh has %d -> %s:%d but not in registry\n",
				act.ExternalPort, act.TargetIP, act.InternalPort)
			unregistered++
		}
	}
	
	if orphaned == 0 && unregistered == 0 {
		fmt.Println("  ✅ Port proxy registry matches netsh state")
	} else {
		fmt.Printf("  Found %d orphaned and %d unregistered port proxy entries\n", orphaned, unregistered)
	}
	
	return nil
}

// auditFirewallRules checks firewall rule registry vs actual Windows Firewall state
func (rm *RegistryManager) auditFirewallRules() error {
	registered, err := rm.GetRegisteredFirewallRules()
	if err != nil {
		return err
	}
	
	// Get actual firewall rules using netsh (similar to existing validation logic)
	actualRules, err := getActualFirewallRules()
	if err != nil {
		return err
	}
	
	// Check for orphaned registry entries
	orphaned := 0
	for _, reg := range registered {
		found := false
		for _, act := range actualRules {
			if reg.RuleName == act {
				found = true
				break
			}
		}
		if !found {
			fmt.Printf("  ORPHANED: Registry has firewall rule '%s' but not found in system\n", reg.RuleName)
			orphaned++
		}
	}
	
	// Check for unregistered actual rules (only WSL2-related)
	unregistered := 0
	for _, act := range actualRules {
		if strings.Contains(act, "WSL2") {
			found := false
			for _, reg := range registered {
				if reg.RuleName == act {
					found = true
					break
				}
			}
			if !found {
				fmt.Printf("  UNREGISTERED: System has WSL2 firewall rule '%s' but not in registry\n", act)
				unregistered++
			}
		}
	}
	
	if orphaned == 0 && unregistered == 0 {
		fmt.Println("  ✅ Firewall rule registry matches system state")
	} else {
		fmt.Printf("  Found %d orphaned and %d unregistered firewall rule entries\n", orphaned, unregistered)
	}
	
	return nil
}

// CleanupOrphanedEntries removes registry entries that don't have corresponding system resources
func (rm *RegistryManager) CleanupOrphanedEntries() error {
	fmt.Println("=== Cleaning Up Orphaned Registry Entries ===")
	
	totalCleaned := 0
	
	// Cleanup orphaned port proxy entries
	if cleaned, err := rm.cleanupOrphanedPortProxies(); err != nil {
		return fmt.Errorf("failed to cleanup port proxy entries: %v", err)
	} else {
		totalCleaned += cleaned
	}
	
	// Cleanup orphaned firewall rule entries
	if cleaned, err := rm.cleanupOrphanedFirewallRules(); err != nil {
		return fmt.Errorf("failed to cleanup firewall rule entries: %v", err)
	} else {
		totalCleaned += cleaned
	}
	
	fmt.Printf("\n✅ Cleaned up %d orphaned registry entries\n", totalCleaned)
	return nil
}

// cleanupOrphanedPortProxies removes port proxy registry entries without corresponding netsh entries
func (rm *RegistryManager) cleanupOrphanedPortProxies() (int, error) {
	registered, err := rm.GetRegisteredPortProxies()
	if err != nil {
		return 0, err
	}
	
	service := &ServiceState{}
	actual, err := service.getCurrentPortMappings()
	if err != nil {
		return 0, err
	}
	
	cleaned := 0
	for _, reg := range registered {
		found := false
		for _, act := range actual {
			if reg.ListenPort == act.ExternalPort &&
				reg.ConnectAddress == act.TargetIP &&
				reg.ConnectPort == act.InternalPort {
				found = true
				break
			}
		}
		if !found {
			fmt.Printf("  Removing orphaned port proxy registry entry: %s\n", reg.Key)
			if err := registry.DeleteKey(rm.portProxyKey, reg.Key); err != nil {
				log.Printf("Warning: failed to delete orphaned port proxy entry %s: %v", reg.Key, err)
			} else {
				cleaned++
			}
		}
	}
	
	return cleaned, nil
}

// cleanupOrphanedFirewallRules removes firewall rule registry entries without corresponding system rules
func (rm *RegistryManager) cleanupOrphanedFirewallRules() (int, error) {
	registered, err := rm.GetRegisteredFirewallRules()
	if err != nil {
		return 0, err
	}
	
	actualRules, err := getActualFirewallRules()
	if err != nil {
		return 0, err
	}
	
	cleaned := 0
	for _, reg := range registered {
		found := false
		for _, act := range actualRules {
			if reg.RuleName == act {
				found = true
				break
			}
		}
		if !found {
			fmt.Printf("  Removing orphaned firewall rule registry entry: %s\n", reg.Key)
			if err := registry.DeleteKey(rm.firewallRuleKey, reg.Key); err != nil {
				log.Printf("Warning: failed to delete orphaned firewall rule entry %s: %v", reg.Key, err)
			} else {
				cleaned++
			}
		}
	}
	
	return cleaned, nil
}

// getActualFirewallRules retrieves the names of all existing firewall rules
func getActualFirewallRules() ([]string, error) {
	rules := []string{}
	
	// This is a simplified version - in practice you might want to use the same
	// netsh parsing logic as in the existing checkFirewallRules function
	cmd := exec.Command("netsh", "advfirewall", "firewall", "show", "rule", "name=all")
	output, err := cmd.Output()
	if err != nil {
		return rules, fmt.Errorf("failed to get firewall rules: %v", err)
	}
	
	outputStr, err := decodeCommandOutput(output)
	if err != nil {
		return rules, fmt.Errorf("failed to decode firewall rules output: %v", err)
	}
	
	lines := strings.Split(outputStr, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Rule Name:") {
			ruleName := strings.TrimPrefix(line, "Rule Name:")
			ruleName = strings.TrimSpace(ruleName)
			if ruleName != "" {
				rules = append(rules, ruleName)
			}
		}
	}
	
	return rules, nil
}