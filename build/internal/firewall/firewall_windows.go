package firewall

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// EnsureAppAllowed creates a Windows Firewall rule to allow the current application
func EnsureAppAllowed() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Get absolute path
	exePath, err = filepath.Abs(exePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	ruleName := "miniVPN"

	// Check if rule already exists
	checkCmd := exec.Command("netsh", "advfirewall", "firewall", "show", "rule", fmt.Sprintf("name=%s", ruleName))
	output, _ := checkCmd.CombinedOutput()

	if strings.Contains(string(output), ruleName) {
		// Rule exists, check if it's for the correct path
		if strings.Contains(string(output), exePath) {
			// Rule exists and is correct
			return nil
		}
		// Delete old rule
		deleteCmd := exec.Command("netsh", "advfirewall", "firewall", "delete", "rule", fmt.Sprintf("name=%s", ruleName))
		deleteCmd.Run()
	}

	// Create inbound rule for UDP
	inboundUDP := exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
		fmt.Sprintf("name=%s", ruleName),
		"dir=in",
		"action=allow",
		fmt.Sprintf("program=%s", exePath),
		"protocol=UDP",
		"enable=yes",
		"profile=any",
	)
	if err := inboundUDP.Run(); err != nil {
		return fmt.Errorf("failed to create inbound UDP rule: %w", err)
	}

	// Create inbound rule for TCP
	inboundTCP := exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
		fmt.Sprintf("name=%s TCP", ruleName),
		"dir=in",
		"action=allow",
		fmt.Sprintf("program=%s", exePath),
		"protocol=TCP",
		"enable=yes",
		"profile=any",
	)
	if err := inboundTCP.Run(); err != nil {
		return fmt.Errorf("failed to create inbound TCP rule: %w", err)
	}

	return nil
}

// RemoveAppRules removes the firewall rules created by this app
func RemoveAppRules() error {
	ruleName := "miniVPN"

	// Delete UDP rule
	deleteUDP := exec.Command("netsh", "advfirewall", "firewall", "delete", "rule", fmt.Sprintf("name=%s", ruleName))
	deleteUDP.Run()

	// Delete TCP rule
	deleteTCP := exec.Command("netsh", "advfirewall", "firewall", "delete", "rule", fmt.Sprintf("name=%s TCP", ruleName))
	deleteTCP.Run()

	return nil
}
