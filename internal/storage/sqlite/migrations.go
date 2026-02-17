package sqlite

import "database/sql"

func migrate(db *sql.DB) error {
	const op = "storage.sqlite.migrate"

	_, err := db.Exec(`
  	CREATE TABLE IF NOT EXISTS sites (
          id INTEGER PRIMARY KEY AUTOINCREMENT,
          name TEXT NOT NULL,
          url TEXT NOT NULL,
          status TEXT NOT NULL DEFAULT 'unknown',
          ping_interval INTEGER NOT NULL DEFAULT 30,
          http_interval INTEGER NOT NULL DEFAULT 300,
          browser_interval INTEGER NOT NULL DEFAULT 1800,
          next_ping_at DATETIME,
          next_http_at DATETIME,
          next_browser_at DATETIME
      );

      CREATE TABLE IF NOT EXISTS checks (
          id INTEGER PRIMARY KEY AUTOINCREMENT,
          site_id INTEGER NOT NULL REFERENCES sites(id),
          step_type TEXT NOT NULL,
          status TEXT NOT NULL,
          status_code INTEGER,
          response_time INTEGER,
          error_message TEXT,
          screenshot_path TEXT,
          attempt INTEGER NOT NULL DEFAULT 1,
          started_at DATETIME NOT NULL,
          finished_at DATETIME NOT NULL
      );

      CREATE TABLE IF NOT EXISTS incidents (
          id INTEGER PRIMARY KEY AUTOINCREMENT,
          site_id INTEGER NOT NULL REFERENCES sites(id),
          status TEXT NOT NULL DEFAULT 'open',
          opened_at DATETIME NOT NULL,
          resolved_at DATETIME,
          screenshot_path TEXT
      );

      CREATE INDEX IF NOT EXISTS idx_checks_site_id ON checks(site_id);
      CREATE INDEX IF NOT EXISTS idx_incidents_site_id_status ON incidents(site_id, status);
	`)

	return err
}
