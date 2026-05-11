package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"

	"github.com/spf13/cobra"
)

const systemdTemplate = `[Unit]
Description=configdrift background management daemon
After=network.target

[Service]
ExecStart={{.BinaryPath}} daemon
Restart=on-failure
Environment=PATH=/usr/bin:/usr/local/bin:{{.HomeDir}}/bin
WorkingDirectory={{.HomeDir}}

[Install]
WantedBy=default.target
`

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage the configdrift background service",
}

var serviceInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install and start the configdrift daemon as a systemd user service",
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}

		// 1. Find binary path
		binaryPath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to get executable path: %w", err)
		}
		binaryPath, _ = filepath.Abs(binaryPath)

		// 2. Ensure systemd user directory exists
		systemdDir := filepath.Join(home, ".config", systemdTemplateDir)
		if err := os.MkdirAll(systemdDir, 0o755); err != nil {
			return fmt.Errorf("failed to create systemd directory: %w", err)
		}

		unitFile := filepath.Join(systemdDir, "configdrift.service")
		fmt.Printf("Generating service file at %s...\n", unitFile)

		// 3. Generate unit file
		f, err := os.Create(unitFile)
		if err != nil {
			return fmt.Errorf("failed to create unit file: %w", err)
		}
		defer f.Close()

		tmpl, err := template.New("systemd").Parse(systemdTemplate)
		if err != nil {
			return err
		}

		data := struct {
			BinaryPath string
			HomeDir    string
		}{
			BinaryPath: binaryPath,
			HomeDir:    home,
		}

		if err := tmpl.Execute(f, data); err != nil {
			return fmt.Errorf("failed to write unit file: %w", err)
		}

		// 4. Enable and start service via systemctl --user
		fmt.Println("Activating service via systemctl --user...")

		commands := [][]string{
			{"daemon-reload"},
			{"enable", "configdrift.service"},
			{"start", "configdrift.service"},
		}

		for _, args := range commands {
			fullArgs := append([]string{"--user"}, args...)
			c := exec.Command("systemctl", fullArgs...)
			if out, err := c.CombinedOutput(); err != nil {
				return fmt.Errorf("systemctl %v failed: %w (output: %s)", args, err, string(out))
			}
		}

		fmt.Println("Successfully installed and started configdrift daemon!")
		fmt.Println("Check status with: systemctl --user status configdrift.service")
		return nil
	},
}

const systemdTemplateDir = "systemd/user"

func init() {
	serviceCmd.AddCommand(serviceInstallCmd)
	rootCmd.AddCommand(serviceCmd)
}
