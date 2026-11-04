package vm

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	UbuntuCloudImageURL = "https://cloud-images.ubuntu.com/minimal/releases/noble/release/ubuntu-24.04-minimal-cloudimg-arm64.img"
	BaseImageFileName   = "ubuntu-24.04-minimal-cloudimg-arm64.img"
	UEFIFirmwarePath    = "/opt/homebrew/share/qemu/edk2-aarch64-code.fd"
	QEMUBinaryPath      = "/opt/homebrew/bin/qemu-system-aarch64"
	QEMUImgPath         = "/opt/homebrew/bin/qemu-img"
)

type Provisioner struct {
	cacheDir string
}

func NewProvisioner(cacheDir string) *Provisioner {
	_ = os.MkdirAll(cacheDir, 0755)
	return &Provisioner{cacheDir: cacheDir}
}

// EnsureBaseImage downloads the official Ubuntu ARM64 cloud image if not already cached.
func (p *Provisioner) EnsureBaseImage(ctx context.Context) (string, error) {
	targetPath := filepath.Join(p.cacheDir, BaseImageFileName)
	if fi, err := os.Stat(targetPath); err == nil && fi.Size() > 50*1024*1024 {
		log.Printf("📦 Using cached Ubuntu ARM64 cloud image: %s (%.1f MB)", targetPath, float64(fi.Size())/1024/1024)
		return targetPath, nil
	}

	log.Printf("⬇️ Downloading Ubuntu 24.04 ARM64 minimal image from %s ...", UbuntuCloudImageURL)
	tmpPath := targetPath + ".part"
	_ = os.Remove(tmpPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, UbuntuCloudImageURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to start download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	outFile, err := os.Create(tmpPath)
	if err != nil {
		return "", err
	}
	defer outFile.Close()

	written, err := io.Copy(outFile, resp.Body)
	if err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("download copy failed: %w", err)
	}
	_ = outFile.Close()

	if err := os.Rename(tmpPath, targetPath); err != nil {
		return "", fmt.Errorf("failed to rename downloaded image: %w", err)
	}

	log.Printf("✅ Download complete! Saved %s (%.1f MB)", targetPath, float64(written)/1024/1024)
	return targetPath, nil
}

// CreateOverlayDisk creates a sparse copy-on-write qcow2 disk linked to base image.
func (p *Provisioner) CreateOverlayDisk(baseImage, diskPath string, sizeGB int) error {
	_ = os.MkdirAll(filepath.Dir(diskPath), 0755)
	if _, err := os.Stat(diskPath); err == nil {
		return nil // already exists, preserve persistent storage!
	}

	if sizeGB <= 0 {
		sizeGB = 20
	}

	log.Printf("💾 Creating persistent sparse qcow2 disk (%d GB): %s", sizeGB, diskPath)
	cmd := exec.Command(QEMUImgPath, "create", "-f", "qcow2", "-b", baseImage, "-F", "qcow2", diskPath, fmt.Sprintf("%dG", sizeGB))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("qemu-img create failed: %s (%w)", string(out), err)
	}
	return nil
}

// GenerateCloudInitISO creates a standard cidata.iso using native macOS hdiutil.
func (p *Provisioner) GenerateCloudInitISO(isoPath, instanceID, hostname, username, password, token string) error {
	_ = os.MkdirAll(filepath.Dir(isoPath), 0755)
	if _, err := os.Stat(isoPath); err == nil {
		return nil // already generated
	}

	tmpDir, err := os.MkdirTemp("", "macpilot-cidata-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	metaData := fmt.Sprintf("instance-id: %s\nlocal-hostname: %s\n", instanceID, hostname)
	if err := os.WriteFile(filepath.Join(tmpDir, "meta-data"), []byte(metaData), 0644); err != nil {
		return err
	}

	userData := fmt.Sprintf(`#cloud-config
hostname: %s
manage_etc_hosts: true

users:
  - name: %s
    gecos: MacPilot Operator
    primary_group: %s
    groups: [sudo, adm, cdrom, dip, plugdev]
    sudo: ALL=(ALL) NOPASSWD:ALL
    shell: /bin/bash
    lock_passwd: false
    plain_text_passwd: "%s"

chpasswd:
  list: |
    %s:%s
  expire: false

package_update: false
package_upgrade: false

write_files:
  - path: /etc/systemd/system/macpilot-guest-agent.service
    permissions: '0644'
    content: |
      [Unit]
      Description=MacPilot AI Guest Agent Daemon
      After=network.target network-online.target

      [Service]
      Type=simple
      User=root
      ExecStart=/usr/local/bin/macpilot-guest-agent --port 8090 --token %s
      Restart=always
      RestartSec=2
      StandardOutput=journal
      StandardError=journal

      [Install]
      WantedBy=multi-user.target

  - path: /usr/local/bin/setup-macpilot.sh
    permissions: '0755'
    content: |
      #!/bin/bash
      set -e
      echo "🚀 Installing MacPilot Guest Agent..."
      for i in $(seq 1 30); do
        if curl -sSf http://10.0.2.2:8089/api/v1/guest-agent/binary -o /usr/local/bin/macpilot-guest-agent; then
          chmod +x /usr/local/bin/macpilot-guest-agent
          systemctl daemon-reload
          systemctl enable --now macpilot-guest-agent.service
          echo "✅ MacPilot Guest Agent service started!"
          exit 0
        fi
        sleep 2
      done

runcmd:
  - /usr/local/bin/setup-macpilot.sh &
`, hostname, username, username, password, username, password, token)

	if err := os.WriteFile(filepath.Join(tmpDir, "user-data"), []byte(userData), 0644); err != nil {
		return err
	}

	// Build ISO via macOS hdiutil
	tmpISO := isoPath + ".tmp.iso"
	_ = os.Remove(tmpISO)
	cmd := exec.Command("hdiutil", "makehybrid", "-iso", "-joliet", "-default-volume-name", "cidata", "-o", tmpISO, tmpDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("hdiutil makehybrid failed: %s (%w)", string(out), err)
	}

	if err := os.Rename(tmpISO, isoPath); err != nil {
		return err
	}

	log.Printf("💿 Cloud-Init seed ISO generated at: %s", isoPath)
	return nil
}
