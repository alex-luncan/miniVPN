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

	ruleNameUDP := "miniVPN UDP"
	ruleNameTCP := "miniVPN TCP"

	// Check if UDP rule already exists with correct path
	udpExists := checkRuleExists(ruleNameUDP, exePath)
	tcpExists := checkRuleExists(ruleNameTCP, exePath)

	// Both rules exist and are correct - skip
	if udpExists && tcpExists {
		return nil
	}

	// Create UDP rule if needed
	if !udpExists {
		inboundUDP := exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
			fmt.Sprintf("name=%s", ruleNameUDP),
			"dir=in",
			"action=allow",
			fmt.Sprintf("program=%s", exePath),
			"protocol=UDP",
			"enable=yes",
			"profile=any",
		)
		inboundUDP.Run() // Ignore errors - may fail if not admin
	}

	// Create TCP rule if needed
	if !tcpExists {
		inboundTCP := exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
			fmt.Sprintf("name=%s", ruleNameTCP),
			"dir=in",
			"action=allow",
			fmt.Sprintf("program=%s", exePath),
			"protocol=TCP",
			"enable=yes",
			"profile=any",
		)
		inboundTCP.Run() // Ignore errors - may fail if not admin
	}

	return nil
}

// checkRuleExists checks if a firewall rule exists for the given program
func checkRuleExists(ruleName, exePath string) bool {
	checkCmd := exec.Command("netsh", "advfirewall", "firewall", "show", "rule", fmt.Sprintf("name=%s", ruleName))
	output, err := checkCmd.CombinedOutput()
	if err != nil {
		return false
	}

	outputStr := string(output)
	// Rule exists if we find the rule name AND the correct program path
	return strings.Contains(outputStr, ruleName) && strings.Contains(strings.ToLower(outputStr), strings.ToLower(exePath))
}

// RemoveAppRules removes the firewall rules created by this app
func RemoveAppRules() error {
	// Delete UDP rule
	deleteUDP := exec.Command("netsh", "advfirewall", "firewall", "delete", "rule", "name=miniVPN UDP")
	deleteUDP.Run()

	// Delete TCP rule
	deleteTCP := exec.Command("netsh", "advfirewall", "firewall", "delete", "rule", "name=miniVPN TCP")
	deleteTCP.Run()

	// Also clean up old rule names if they exist
	exec.Command("netsh", "advfirewall", "firewall", "delete", "rule", "name=miniVPN").Run()

	return nil
}
