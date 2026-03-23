package splittunnel

import (
	"fmt"
	"net"
	"sync"
)

// Protocol defines the network protocol
type Protocol int

const (
	ProtocolTCP Protocol = iota
	ProtocolUDP
	ProtocolBoth
)

func (p Protocol) String() string {
	switch p {
	case ProtocolTCP:
		return "TCP"
	case ProtocolUDP:
		return "UDP"
	case ProtocolBoth:
		return "TCP/UDP"
	default:
		return "Unknown"
	}
}

// Direction defines traffic direction
type Direction int

const (
	DirectionInbound Direction = iota
	DirectionOutbound
	DirectionBoth
)

// RuleAction defines what to do with matching traffic
type RuleAction int

const (
	ActionRoute  RuleAction = iota // Route through VPN
	ActionBypass                   // Bypass VPN, use normal network
)

// Rule defines a routing rule
type Rule struct {
	ID        uint64
	Port      uint16
	Protocol  Protocol
	Direction Direction
	Action    RuleAction
	Gateway   net.IP
	Interface string
	Priority  int
}

// Router manages routing rules
type Router struct {
	rules    map[uint64]Rule
	nextID   uint64
	active   bool
	mu       sync.RWMutex
	platform PlatformRouter
}

// PlatformRouter is the interface for platform-specific routing
type PlatformRouter interface {
	AddRule(rule Rule) error
	RemoveRule(id uint64) error
	ClearAll() error
	IsAvailable() bool
}

// NewRouter creates a new router
func NewRouter() *Router {
	r := &Router{
		rules:  make(map[uint64]Rule),
		nextID: 1,
	}

	// Initialize platform-specific router
	r.platform = newPlatformRouter()

	return r
}

// ApplyRules applies a set of rules
func (r *Router) ApplyRules(rules []Rule) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Clear existing rules first
	if err := r.clearAllLocked(); err != nil {
		return fmt.Errorf("failed to clear existing rules: %w", err)
	}

	// Apply new rules
	for _, rule := range rules {
		rule.ID = r.nextID
		r.nextID++

		if r.platform != nil && r.platform.IsAvailable() {
			if err := r.platform.AddRule(rule); err != nil {
				return fmt.Errorf("failed to add rule for port %d: %w", rule.Port, err)
			}
		}

		r.rules[rule.ID] = rule
	}

	r.active = true
	return nil
}

// AddRule adds a single rule
func (r *Router) AddRule(rule Rule) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	rule.ID = r.nextID
	r.nextID++

	if r.platform != nil && r.platform.IsAvailable() {
		if err := r.platform.AddRule(rule); err != nil {
			return fmt.Errorf("failed to add rule: %w", err)
		}
	}

	r.rules[rule.ID] = rule
	return nil
}

// RemoveRule removes a rule by ID
func (r *Router) RemoveRule(id uint64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.rules[id]; !exists {
		return fmt.Errorf("rule %d not found", id)
	}

	if r.platform != nil && r.platform.IsAvailable() {
		if err := r.platform.RemoveRule(id); err != nil {
			return fmt.Errorf("failed to remove rule: %w", err)
		}
	}

	delete(r.rules, id)
	return nil
}

// ClearRules removes all rules
func (r *Router) ClearRules() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.clearAllLocked()
}

// clearAllLocked clears all rules (must be called with lock held)
func (r *Router) clearAllLocked() error {
	if r.platform != nil && r.platform.IsAvailable() {
		if err := r.platform.ClearAll(); err != nil {
			return fmt.Errorf("failed to clear platform rules: %w", err)
		}
	}

	r.rules = make(map[uint64]Rule)
	r.active = false
	return nil
}

// GetRules returns all active rules
func (r *Router) GetRules() []Rule {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rules := make([]Rule, 0, len(r.rules))
	for _, rule := range r.rules {
		rules = append(rules, rule)
	}
	return rules
}

// GetRuleByPort returns the rule for a specific port
func (r *Router) GetRuleByPort(port uint16) *Rule {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, rule := range r.rules {
		if rule.Port == port {
			ruleCopy := rule
			return &ruleCopy
		}
	}
	return nil
}

// IsActive returns whether the router has active rules
func (r *Router) IsActive() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.active
}

// RuleCount returns the number of active rules
func (r *Router) RuleCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.rules)
}

// MatchPacket determines if a packet matches any rule
func (r *Router) MatchPacket(port uint16, protocol Protocol, direction Direction) *Rule {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, rule := range r.rules {
		// Check port
		if rule.Port != port {
			continue
		}

		// Check protocol
		if rule.Protocol != ProtocolBoth && rule.Protocol != protocol {
			continue
		}

		// Check direction
		if rule.Direction != DirectionBoth && rule.Direction != direction {
			continue
		}

		ruleCopy := rule
		return &ruleCopy
	}

	return nil
}
