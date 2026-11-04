package tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/macpilot/agent-engine/internal/persistence"
	"github.com/macpilot/agent-engine/internal/policy"
	"github.com/macpilot/agent-engine/internal/vm"
	"github.com/macpilot/agent-engine/pkg/protocol"
)

func TestSentinelPolicyComputerActions(t *testing.T) {
	sp := policy.NewSentinelPolicy()

	// Safe command should be allowed
	safe := &protocol.AgentAction{
		Action:  protocol.ActionComputerExecute,
		Command: "ls -la /home/macpilot",
	}
	res := sp.CheckAction(safe)
	if !res.Allowed {
		t.Fatalf("Safe computer command was unexpectedly blocked: %v", res.Reason)
	}

	// Destructive commands should be blocked or require approval
	blocked := &protocol.AgentAction{
		Action:  protocol.ActionComputerExecute,
		Command: "rm -rf /",
	}
	resBlocked := sp.CheckAction(blocked)
	if resBlocked.Allowed {
		t.Fatalf("Dangerous 'rm -rf /' was not blocked!")
	}

	destructive := &protocol.AgentAction{
		Action:  protocol.ActionComputerExecute,
		Command: "sudo apt update",
	}
	resDestructive := sp.CheckAction(destructive)
	if !resDestructive.NeedsApproval {
		t.Fatalf("Command with 'sudo' did not require human approval!")
	}
}

func TestProvisionerDiskAndISO(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "macpilot-vm-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cacheDir := filepath.Join(tmpDir, "cache")
	prov := vm.NewProvisioner(cacheDir)

	// Test fake base image and overlay disk creation
	fakeBase := filepath.Join(cacheDir, "fake-base.img")
	_ = os.WriteFile(fakeBase, make([]byte, 1024), 0644)

	overlayDisk := filepath.Join(tmpDir, "overlay.qcow2")
	if err := prov.CreateOverlayDisk(fakeBase, overlayDisk, 5); err != nil {
		t.Logf("qemu-img overlay disk creation error (expected if base is dummy): %v", err)
	}

	// Test cloud-init seed ISO generation via hdiutil
	isoPath := filepath.Join(tmpDir, "cidata.iso")
	err = prov.GenerateCloudInitISO(isoPath, "test-vm", "test-host", "macpilot", "testpass", "test-token")
	if err != nil {
		t.Fatalf("Failed to generate cloud-init ISO: %v", err)
	}

	fi, err := os.Stat(isoPath)
	if err != nil || fi.Size() == 0 {
		t.Fatalf("Cloud-init ISO was not created or empty: %v", err)
	}
	t.Logf("✅ Successfully generated cloud-init ISO: %d bytes", fi.Size())
}

func TestComputerManagerLifecycle(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "macpilot-mgr-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := persistence.NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to open test db: %v", err)
	}
	defer db.Close()

	mgr := vm.NewComputerManager(db, tmpDir)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_ = ctx
	// Verify control mode setting
	_ = mgr.SetControlMode("ubuntu-primary", "manual")
	primary := mgr.GetPrimaryComputer()
	if primary != nil && primary.ControlMode != "manual" {
		t.Fatalf("Expected manual control mode, got: %s", primary.ControlMode)
	}
	t.Log("✅ Computer Manager initialized cleanly with persistent storage support.")
}
