package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"
)

const systemdUnit = `[Unit]
Description=vefr IPv6 HTTP forward proxy
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=vefr
Group=vefr
ExecStart=%s run --config %s
Restart=on-failure
RestartSec=5s
LimitNOFILE=65536
NoNewPrivileges=true
PrivateTmp=true
ProtectHome=true
ProtectSystem=strict
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX
LockPersonality=true
MemoryDenyWriteExecute=true
ReadOnlyPaths=%s %s

[Install]
WantedBy=multi-user.target
`

func newSystemdCommand() *cobra.Command {
	systemd := &cobra.Command{Use: "systemd", Short: "Manage the systemd service"}
	var unitPath, binaryPath string
	var enable bool
	install := &cobra.Command{
		Use: "install", Short: "Install or update the systemd service (requires sudo)", Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			if configPath == "config.toml" {
				configPath = "/etc/vefr/config.toml"
			}
			return installSystemd(unitPath, binaryPath, configPath, enable)
		},
	}
	install.Flags().StringVar(&unitPath, "unit", "/etc/systemd/system/vefr.service", "systemd unit path")
	install.Flags().StringVar(&binaryPath, "binary", "/usr/local/bin/vefr", "installed binary path")
	install.Flags().BoolVar(&enable, "enable", false, "enable the service at boot (does not start it)")
	systemd.AddCommand(install)
	return systemd
}

func installSystemd(unitPath, binaryPath, configPath string, enable bool) error {
	if os.Geteuid() != 0 {
		return errors.New("systemd install requires root; re-run with sudo")
	}
	if filepath.IsAbs(configPath) == false {
		return errors.New("systemd config path must be absolute")
	}
	if err := os.MkdirAll(filepath.Dir(unitPath), 0755); err != nil {
		return fmt.Errorf("create unit directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0750); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	if err := ensureServiceAccount(); err != nil {
		return err
	}
	if err := os.Chown(filepath.Dir(configPath), 0, serviceGroupID()); err != nil {
		return fmt.Errorf("set config directory group: %w", err)
	}
	if err := installSelf(binaryPath); err != nil {
		return err
	}
	if _, err := os.Stat(configPath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("check config: %w", err)
		}
	} else {
		if err := os.Chown(configPath, 0, serviceGroupID()); err != nil {
			return fmt.Errorf("set config ownership: %w", err)
		}
		if err := os.Chmod(configPath, 0640); err != nil {
			return fmt.Errorf("set config permissions: %w", err)
		}
	}
	unit := fmt.Sprintf(systemdUnit, binaryPath, configPath, filepath.Dir(configPath), binaryPath)
	if err := os.WriteFile(unitPath, []byte(unit), 0644); err != nil {
		return fmt.Errorf("write systemd unit: %w", err)
	}
	if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("systemctl daemon-reload: %w", err)
	}
	if enable {
		if err := exec.Command("systemctl", "enable", "vefr.service").Run(); err != nil {
			return fmt.Errorf("enable service: %w", err)
		}
	}
	fmt.Printf("systemd service installed at %s\n", unitPath)
	return nil
}

func ensureServiceAccount() error {
	if err := exec.Command("getent", "group", "vefr").Run(); err != nil {
		if err := exec.Command("groupadd", "--system", "vefr").Run(); err != nil {
			return fmt.Errorf("create vefr group: %w", err)
		}
	}
	if err := exec.Command("id", "-u", "vefr").Run(); err != nil {
		if err := exec.Command("useradd", "--system", "--home-dir", "/var/lib/vefr", "--create-home", "--shell", "/usr/sbin/nologin", "--gid", "vefr", "vefr").Run(); err != nil {
			return fmt.Errorf("create vefr user: %w", err)
		}
	}
	return nil
}

func installSelf(destination string) error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find executable: %w", err)
	}
	if filepath.Clean(executable) == filepath.Clean(destination) {
		return nil
	}
	data, err := os.ReadFile(executable)
	if err != nil {
		return fmt.Errorf("read executable: %w", err)
	}
	if err := os.WriteFile(destination, data, 0755); err != nil {
		return fmt.Errorf("install executable: %w", err)
	}
	if err := os.Chmod(destination, 0755); err != nil {
		return fmt.Errorf("set executable permissions: %w", err)
	}
	return nil
}

func serviceGroupID() int {
	group, err := user.LookupGroup("vefr")
	if err != nil {
		return 0
	}
	id, err := strconv.Atoi(group.Gid)
	if err != nil {
		return 0
	}
	return id
}
