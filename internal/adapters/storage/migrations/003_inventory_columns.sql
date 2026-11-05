CREATE TABLE account_inventory_preferences (
 account_id INTEGER PRIMARY KEY REFERENCES account(id),
 columns_json TEXT NOT NULL
);
