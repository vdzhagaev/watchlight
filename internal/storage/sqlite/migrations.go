package sqlite

import (
	"database/sql"
	"fmt"
)

func migrate(db *sql.DB) error {
	const op = "storage.sqlite.migrate"

	q := `
  	CREATE TABLE IF NOT EXISTS monitors (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        name TEXT NOT NULL,
        url TEXT NOT NULL UNIQUE,
        status TEXT NOT NULL DEFAULT 'unknown'
    );

    CREATE TABLE IF NOT EXISTS monitor_check_configs (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        monitor_id INTEGER NOT NULL,
        check_type TEXT NOT NULL,
        is_enabled BOOLEAN NOT NULL DEFAULT(1),
        check_interval INTEGER NOT NULL DEFAULT 60,
        max_attempts INTEGER NOT NULL DEFAULT 3,
        do_error_screenshot BOOLEAN NOT NULL DEFAULT 0,
        keywords TEXT,

        FOREIGN KEY (monitor_id) REFERENCES monitors(id) ON DELETE CASCADE
    );

    CREATE TABLE IF NOT EXISTS monitor_check_results (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        monitor_id INTEGER NOT NULL,
        config_id INTEGER NOT NULL,
        status TEXT NOT NULL,
        status_code INTEGER,
        response_time_ns INTEGER,
        checked_at DATETIME NOT NULL,
        error_message TEXT,
        screenshot_path TEXT,

        FOREIGN KEY (monitor_id) REFERENCES monitors(id) ON DELETE CASCADE,
        FOREIGN KEY (config_id) REFERENCES monitor_check_configs(id) ON DELETE CASCADE
    );

    CREATE INDEX IF NOT EXISTS idx_results_monitor_time ON monitor_check_results(monitor_id, checked_at);
    CREATE INDEX IF NOT EXISTS idx_results_config ON monitor_check_results(config_id);
	`

	_, err := db.Exec(q)

	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}
