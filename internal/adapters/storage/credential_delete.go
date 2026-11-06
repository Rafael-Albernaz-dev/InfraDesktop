package storage

import (
	"context"
	"database/sql"
	"infradesktop/internal/modules/equipment"
)

func deleteRemovedCredentials(ctx context.Context, tx *sql.Tx, old, current []equipment.SealedCredential) equipment.Code {
	present := make(map[string]bool, len(current))
	for _, c := range current {
		present[c.Metadata.ID] = true
	}
	for _, c := range old {
		if !present[c.Metadata.ID] {
			if _, err := tx.ExecContext(ctx, "DELETE FROM credentials WHERE id=?", c.Metadata.ID); err != nil {
				return equipment.Unavailable
			}
		}
	}
	return equipment.OK
}
