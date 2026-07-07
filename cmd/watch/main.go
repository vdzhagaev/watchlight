// Command watch streams check_results as the running server's scheduler records
// them, joined with monitor/config details for a readable live feed. Temporary
// dev tool (future TUI). Ctrl+C to stop.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"time"

	_ "modernc.org/sqlite"
)

const (
	green = "\033[32m"
	red   = "\033[31m"
	dim   = "\033[2m"
	bold  = "\033[1m"
	reset = "\033[0m"
)

type row struct {
	id, status, host, errMsg string
	scheme, path, method     sql.NullString
	port                     sql.NullInt64
	statusCode, respNs       sql.NullInt64
	checkedAt                string
	isHTTP                   bool
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: watch <db-path>")
		os.Exit(1)
	}
	dbPath := os.Args[1]

	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)")
	if err != nil {
		fmt.Fprintln(os.Stderr, "open db:", err)
		os.Exit(1)
	}
	defer db.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	seen := map[string]bool{}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	fmt.Printf("%swatching check_results — %s (Ctrl+C to stop)%s\n", bold, dbPath, reset)
	fmt.Printf("%s%-12s %-9s %-5s  %-40s %6s %10s%s\n", dim, "TIME", "STATUS", "KIND", "TARGET", "CODE", "LATENCY", reset)

	for {
		poll(ctx, db, seen)
		select {
		case <-ctx.Done():
			fmt.Println("\nstopped.")
			return
		case <-ticker.C:
		}
	}
}

func poll(ctx context.Context, db *sql.DB, seen map[string]bool) {
	const q = `
		SELECT cr.id, cr.checked_at, cr.status, cr.status_code, cr.response_time_ns,
		       COALESCE(cr.error_message, ''), m.host,
		       cr.http_config_id IS NOT NULL,
		       pc.port, hc.scheme, hc.path, hc.method
		FROM check_results cr
		JOIN monitors m ON m.id = cr.monitor_id
		LEFT JOIN ping_configs pc ON pc.id = cr.ping_config_id
		LEFT JOIN http_configs hc ON hc.id = cr.http_config_id
		ORDER BY cr.checked_at`
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		fmt.Fprintln(os.Stderr, "query:", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.checkedAt, &r.status, &r.statusCode, &r.respNs,
			&r.errMsg, &r.host, &r.isHTTP, &r.port, &r.scheme, &r.path, &r.method); err != nil {
			fmt.Fprintln(os.Stderr, "scan:", err)
			return
		}
		if seen[r.id] {
			continue
		}
		seen[r.id] = true
		fmt.Println(format(r))
	}
}

func format(r row) string {
	statusCol, symbol := green, "✓"
	if r.status != "success" {
		statusCol, symbol = red, "✗"
	}

	kind, target, code := "ping", fmt.Sprintf("%s:%d", r.host, r.port.Int64), "-"
	if r.isHTTP {
		kind = "http"
		target = fmt.Sprintf("%s %s://%s%s", r.method.String, r.scheme.String, r.host, r.path.String)
		if r.statusCode.Int64 > 0 {
			code = fmt.Sprintf("%d", r.statusCode.Int64)
		}
	}

	line := fmt.Sprintf("%-12s %s%-1s %-7s%s %-5s  %-40s %6s %10s",
		clock(r.checkedAt),
		statusCol, symbol, r.status, reset,
		kind, target, code, latency(r.respNs.Int64))

	if r.errMsg != "" {
		line += fmt.Sprintf("  %s%s%s", dim, r.errMsg, reset)
	}
	return line
}

// clock renders just the wall-clock time from a stored timestamp, falling back
// to the raw value if it doesn't parse.
func clock(ts string) string {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05.999999999-07:00", "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, ts); err == nil {
			return t.Local().Format("15:04:05.000")
		}
	}
	return ts
}

func latency(ns int64) string {
	d := time.Duration(ns)
	if d >= time.Second {
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
	return fmt.Sprintf("%dms", d.Milliseconds())
}
