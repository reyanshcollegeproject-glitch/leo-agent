package policy

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/macpilot/agent-engine/pkg/protocol"
)

type SentinelPolicy struct {
	BlockedPatterns    []*regexp.Regexp
	DestructivePattern []*regexp.Regexp
}

func NewSentinelPolicy() *SentinelPolicy {
	blocked := []string{
		`(?i)\brm\s+-(?:r|f|rf|fr)\s+/(?:\s+|$)`,
		`(?i)\bmkfs\b`,
		`(?i)\bdd\s+if=.*of=/dev/`,
		`(?i):\(\)\{\s*:\|:&\s*\};:`, // Fork bomb
		`(?i)> /dev/sda`,
	}
	destructive := []string{
		`(?i)\brm\s+-`,
		`(?i)\bkillall\b`,
		`(?i)\bpkill\b`,
		`(?i)\bsudo\b`,
		`(?i)\bchmod\s+-R\s+777\b`,
	}

	sp := &SentinelPolicy{}
	for _, p := range blocked {
		sp.BlockedPatterns = append(sp.BlockedPatterns, regexp.MustCompile(p))
	}
	for _, p := range destructive {
		sp.DestructivePattern = append(sp.DestructivePattern, regexp.MustCompile(p))
	}
	return sp
}

type CheckResult struct {
	Allowed          bool
	NeedsApproval    bool
	Reason           string
	Severity         string
}

func (sp *SentinelPolicy) CheckAction(action *protocol.AgentAction) CheckResult {
	if action.Action != protocol.ActionRunCommand && action.Action != protocol.ActionComputerExecute {
		return CheckResult{Allowed: true}
	}

	cmd := strings.TrimSpace(action.Command)

	// Check blocked
	for _, r := range sp.BlockedPatterns {
		if r.MatchString(cmd) {
			return CheckResult{
				Allowed:  false,
				Reason:   fmt.Sprintf("Command blocked by safety sentinel: '%s'", cmd),
				Severity: "critical",
			}
		}
	}

	// Check destructive requiring approval
	for _, r := range sp.DestructivePattern {
		if r.MatchString(cmd) {
			return CheckResult{
				Allowed:       true,
				NeedsApproval: true,
				Reason:        fmt.Sprintf("Destructive command requires human approval: '%s'", cmd),
				Severity:      "high",
			}
		}
	}

	return CheckResult{Allowed: true}
}
