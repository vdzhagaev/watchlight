package sqlite

import (
	"database/sql"
	"fmt"
)

func migrate(db *sql.DB) error {
	const op = "storage.sqlite.migrate"

	q := `
  	CREATE TABLE IF NOT EXISTS monitors (
        id TEXT PRIMARY KEY,
        name TEXT NOT NULL,
        host TEXT NOT NULL UNIQUE,
        status TEXT NOT NULL DEFAULT 'unknown'
    );

    CREATE TABLE IF NOT EXISTS ping_configs (
        id TEXT PRIMARY KEY,
        monitor_id TEXT NOT NULL,
        port INTEGER NOT NULL DEFAULT 443,
        is_enabled BOOLEAN NOT NULL DEFAULT(1),
        interval INTEGER NOT NULL DEFAULT 60,
        timeout INTEGER NOT NULL DEFAULT 10,
        max_attempts INTEGER NOT NULL DEFAULT 3,

        UNIQUE (monitor_id),

        FOREIGN KEY (monitor_id) REFERENCES monitors(id) ON DELETE CASCADE
    );

    CREATE TABLE IF NOT EXISTS http_configs (
        id TEXT PRIMARY KEY,
        monitor_id TEXT NOT NULL,
        scheme TEXT NOT NULL DEFAULT 'https',
        path TEXT NOT NULL DEFAULT '/',
        method TEXT NOT NULL DEFAULT 'HEAD',
        is_enabled BOOLEAN NOT NULL DEFAULT(1),
        interval INTEGER NOT NULL DEFAULT 60,
        timeout INTEGER NOT NULL DEFAULT 10,
        max_attempts INTEGER NOT NULL DEFAULT 3,
        keywords TEXT,

        UNIQUE (monitor_id, scheme, path, method),

        FOREIGN KEY (monitor_id) REFERENCES monitors(id) ON DELETE CASCADE
    );

    CREATE TABLE IF NOT EXISTS check_results (
        id TEXT PRIMARY KEY,
        monitor_id TEXT NOT NULL,
        ping_config_id TEXT NULL,
        http_config_id TEXT NULL,
        status TEXT NOT NULL,
        status_code INTEGER,
        response_time_ns INTEGER,
        checked_at DATETIME NOT NULL,
        error_message TEXT,

        FOREIGN KEY (monitor_id) REFERENCES monitors(id) ON DELETE CASCADE,
        FOREIGN KEY (ping_config_id) REFERENCES ping_configs(id) ON DELETE CASCADE,
        FOREIGN KEY (http_config_id) REFERENCES http_configs(id) ON DELETE CASCADE,
        CHECK (
            (ping_config_id IS NOT NULL) + (http_config_id IS NOT NULL) = 1
        )
    );

    CREATE INDEX IF NOT EXISTS idx_results_monitor_time ON check_results(monitor_id, checked_at DESC);
    CREATE INDEX IF NOT EXISTS idx_results_config ON check_results(ping_config_id) WHERE ping_config_id IS NOT NULL;
    CREATE INDEX IF NOT EXISTS idx_results_http_config ON check_results(http_config_id) WHERE http_config_id IS NOT NULL;
	`

	_, err := db.Exec(q)

	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}
