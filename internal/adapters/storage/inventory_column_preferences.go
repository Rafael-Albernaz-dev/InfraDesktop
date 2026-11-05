package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"infradesktop/internal/modules/access"
)

func loadInventoryColumns(ctx context.Context, q queryer, account *access.Account) error {
	var body string
	err := q.QueryRowContext(ctx, "SELECT columns_json FROM account_inventory_preferences WHERE account_id=1").Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if json.Unmarshal([]byte(body), &account.Columns) != nil || !access.ValidInventoryColumns(account.Columns) {
		return errors.New("invalid inventory column preference")
	}
	return nil
}

func saveInventoryColumns(ctx context.Context, tx *sql.Tx, columns []string) error {
	if columns == nil {
		return nil
	}
	if !access.ValidInventoryColumns(columns) {
		return errors.New("invalid inventory column preference")
	}
	body, err := json.Marshal(columns)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO account_inventory_preferences(account_id,columns_json) VALUES(1,?)
		ON CONFLICT(account_id) DO UPDATE SET columns_json=excluded.columns_json`, string(body))
	return err
}
