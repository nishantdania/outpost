package ark

import (
	"context"
	"fmt"
	"strings"
)

func (s *Store) Schema(ctx context.Context) (string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT sql
		FROM sqlite_master
		WHERE type = 'table' AND name NOT LIKE 'sqlite_%'
		ORDER BY name
	`)
	if err != nil {
		return "", fmt.Errorf("query schema: %w", err)
	}
	defer rows.Close()

	var statements []string
	for rows.Next() {
		var statement string
		if err := rows.Scan(&statement); err != nil {
			return "", fmt.Errorf("scan schema: %w", err)
		}
		statements = append(statements, statement+";")
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("iterate schema: %w", err)
	}

	return strings.Join(statements, "\n\n") + "\n", nil
}
