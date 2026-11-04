package tests

import (
	"testing"

	"github.com/macpilot/agent-engine/internal/policy"
	"github.com/macpilot/agent-engine/pkg/protocol"
)

func TestSentinelPolicyBlockedCommands(t *testing.T) {
	sentinel := policy.NewSentinelPolicy()

	dangerousCommands := []string{
		"rm -rf /",
		"rm -fr /",
		"mkfs.ext4 /dev/sda1",
		":(){ :|:& };:", // fork bomb
	}

	for _, cmd := range dangerousCommands {
		action := &protocol.AgentAction{
			Action:  protocol.ActionRunCommand,
			Command: cmd,
		}
		result := sentinel.CheckAction(action)
		if result.Allowed {
			t.Errorf("Expected command '%s' to be BLOCKED by sentinel, but it was allowed", cmd)
		}
		if result.Severity != "critical" {
			t.Errorf("Expected severity 'critical' for '%s', got '%s'", cmd, result.Severity)
		}
	}
}

func TestSentinelPolicyConsequentialApprovals(t *testing.T) {
	sentinel := policy.NewSentinelPolicy()

	consequentialCommands := []string{
		"rm -f ~/Desktop/test.txt",
		"sudo spctl --master-disable",
		"killall Finder",
	}

	for _, cmd := range consequentialCommands {
		action := &protocol.AgentAction{
			Action:  protocol.ActionRunCommand,
			Command: cmd,
		}
		result := sentinel.CheckAction(action)
		if !result.NeedsApproval {
			t.Errorf("Expected command '%s' to require human approval, but NeedsApproval was false", cmd)
		}
	}
}

func TestSentinelPolicySafeCommands(t *testing.T) {
	sentinel := policy.NewSentinelPolicy()

	safeCommands := []string{
		"open -a 'Google Chrome' 'https://youtube.com'",
		"ls -la ~/Desktop",
		"screencapture -x ~/Desktop/shot.png",
	}

	for _, cmd := range safeCommands {
		action := &protocol.AgentAction{
			Action:  protocol.ActionRunCommand,
			Command: cmd,
		}
		result := sentinel.CheckAction(action)
		if !result.Allowed || result.NeedsApproval {
			t.Errorf("Expected safe command '%s' to be allowed without approval, got allowed=%v needsApproval=%v", cmd, result.Allowed, result.NeedsApproval)
		}
	}
}

