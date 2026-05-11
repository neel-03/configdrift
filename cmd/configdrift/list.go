package main

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/neel-03/configdrift/internal/daemon"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ps"},
	Short:   "List all background watchers managed by the daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := daemon.NewClient("")
		resp, err := client.Send(daemon.Request{Type: daemon.RequestList})
		if err != nil {
			return err
		}

		if !resp.Success {
			return fmt.Errorf("failed to list watchers: %s", resp.Message)
		}

		workersData, ok := resp.Data.([]interface{})
		if !ok && resp.Data != nil {
			return fmt.Errorf("unexpected data format from daemon")
		}

		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(tw, "ID\tCONFIG\tSTATUS\tUPTIME")

		if len(workersData) == 0 {
			fmt.Println("No active background watchers.")
			return nil
		}

		for _, wd := range workersData {
			m, ok := wd.(map[string]interface{})
			if !ok {
				continue
			}

			id, _ := m["id"].(string)
			shortID := id
			if len(id) > 12 {
				shortID = id[:12]
			}
			cfg, _ := m["config_path"].(string)
			status, _ := m["status"].(string)
			startStr, _ := m["start_time"].(string)

			startTime, err := time.Parse(time.RFC3339, startStr)
			uptime := "unknown"
			if err == nil {
				uptime = time.Since(startTime).Round(time.Second).String()
			}

			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", shortID, cfg, status, uptime)
		}

		tw.Flush()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
