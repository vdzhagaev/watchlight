// Command seed inserts one monitor (intrinsic ping + one HTTP check) into the
// sqlite DB so a running server's scheduler starts checking it. Temporary tool
// for manual end-to-end verification.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/vdzhagaev/watchlight/internal/monitor"
	"github.com/vdzhagaev/watchlight/internal/storage/sqlite"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: seed <db-path> <host> [name]")
		os.Exit(1)
	}
	dbPath, host := os.Args[1], os.Args[2]
	name := host
	if len(os.Args) > 3 {
		name = os.Args[3]
	}

	st, err := sqlite.New(dbPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open db:", err)
		os.Exit(1)
	}
	defer st.Close()

	m, err := monitor.New(monitor.CreateMonitorInput{
		Name: name,
		Host: host,
		PingConfig: &monitor.CreatePingConfigInput{
			Port:     443,
			Interval: 10,
			Timeout:  5,
		},
		HTTPConfigs: []monitor.CreateHTTPConfigInput{
			{Scheme: "https", Path: "/", Method: "GET", Interval: 10, Timeout: 5, MaxAttempts: 3, Keywords: []string{"example"}},
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "build monitor:", err)
		os.Exit(1)
	}

	if err := st.CreateMonitor(context.Background(), m); err != nil {
		fmt.Fprintln(os.Stderr, "create monitor:", err)
		os.Exit(1)
	}

	fmt.Printf("seeded monitor id=%s host=%s ping=%s http[0]=%s\n",
		m.ID, m.Host.String(), m.PingConfig.ID, m.HTTPConfigs[0].ID)
}
