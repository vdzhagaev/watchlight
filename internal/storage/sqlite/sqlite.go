package sqlite

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type Storage struct {
	db *sql.DB
}

func New(path string) (*Storage, error) {
	const op = "storage.sqlite.New"

	db, err := sql.Open(
		"sqlite",
		path+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)",
	)

	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	if err := migrate(db); err != nil {
		return nil, err
	}

	return &Storage{db: db}, nil
}

func (s *Storage) Close() error {
	return s.db.Close()
}
