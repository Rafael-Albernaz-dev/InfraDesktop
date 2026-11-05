package storage

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"time"

	"infradesktop/internal/modules/access"
)

func load(ctx context.Context, q queryer) (access.State, error) {
	st := access.State{Companies: []access.Company{}}
	if err := loadAccount(ctx, q, &st); err != nil {
		return st, err
	}
	if err := loadCatalog(ctx, q, &st); err != nil {
		return st, err
	}
	err := loadGrants(ctx, q, &st)
	return st, err
}

func loadAccount(ctx context.Context, q queryer, st *access.State) error {
	var a access.Account
	err := q.QueryRowContext(ctx, "SELECT name,algorithm,memory,iterations,parallelism,salt,hash,"+
		"recovery,epoch,theme,profile FROM account WHERE id=1").Scan(
		&a.Name, &a.Password.Algorithm, &a.Password.Memory, &a.Password.Iterations, &a.Password.Parallelism,
		&a.Password.Salt, &a.Password.Hash, &a.Recovery, &a.Epoch, &a.Theme, &a.Profile)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	v := a.Password
	if v.Algorithm != "argon2id-v19" || v.Memory != 19456 || v.Iterations != 2 || v.Parallelism != 1 ||
		len(v.Salt) != 16 || len(v.Hash) != 32 || len(a.Recovery) != 32 || a.Epoch == 0 ||
		!access.ValidName(a.Name) || (a.Theme != "light" && a.Theme != "dark") {
		return errors.New("invalid account state")
	}
	if err = loadInventoryColumns(ctx, q, &a); err != nil {
		return err
	}
	st.Account = &a
	return nil
}

func loadCatalog(ctx context.Context, q queryer, st *access.State) error {
	var next int64
	if err := q.QueryRowContext(ctx, "SELECT failures,next_attempt FROM attempts WHERE id=1").Scan(&st.Failures, &next); err != nil {
		return err
	}
	if next > 0 {
		st.NextAttempt = time.Unix(0, next)
	}
	rows, err := q.QueryContext(ctx, "SELECT id,name FROM companies ORDER BY rowid")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var c access.Company
		if err = rows.Scan(&c.ID, &c.Name); err != nil {
			return err
		}
		if !access.ValidCompanyID(c.ID) || !access.ValidName(c.Name) {
			return errors.New("invalid catalog")
		}
		st.Companies = append(st.Companies, c)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	if st.Account != nil && st.Account.Profile != "" {
		for _, c := range st.Companies {
			if c.ID == st.Account.Profile {
				return nil
			}
		}
		return errors.New("active company outside catalog")
	}
	return nil
}

func loadGrants(ctx context.Context, q queryer, st *access.State) error {
	rows, err := q.QueryContext(ctx, "SELECT digest,epoch,expires FROM grants")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var g access.Grant
		var expiry int64
		if err = rows.Scan(&g.Digest, &g.Epoch, &expiry); err != nil {
			return err
		}
		g.Expires = time.Unix(0, expiry)
		st.Grants = append(st.Grants, g)
	}
	return rows.Err()
}

func catalogAdditions(ctx context.Context, tx *sql.Tx, companies []access.Company) ([]access.Company, error) {
	var additions []access.Company
	for _, company := range companies {
		var n int
		if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM companies WHERE id=?", company.ID).Scan(&n); err != nil {
			return nil, err
		}
		if n == 0 {
			additions = append(additions, company)
		}
	}
	return additions, nil
}

func (s *Store) prepareCatalogAdditions(ctx context.Context, tx *sql.Tx, additions []access.Company) (*sql.Tx, error) {
	if len(additions) == 0 {
		return tx, nil
	}
	if err := tx.Rollback(); err != nil {
		return nil, err
	}
	for _, company := range additions {
		if err := s.prepareIntent(ctx, company); err != nil {
			return nil, err
		}
	}
	return s.db.BeginTx(ctx, nil)
}

func (s *Store) catalogCandidate(st access.State) (*sql.DB, os.FileInfo, string, error) {
	target := ""
	if st.Account != nil {
		target = st.Account.Profile
	}
	if target == s.unitID || target == "" {
		return nil, nil, target, nil
	}
	p, err := s.unitPath(target)
	if err != nil {
		return nil, nil, target, err
	}
	info, err := os.Lstat(p)
	if err != nil {
		return nil, nil, target, err
	}
	unit, err := s.openUnit(target)
	return unit, info, target, err
}

func persistAccessState(ctx context.Context, tx *sql.Tx, st access.State) error {
	if st.Account != nil {
		a, v := st.Account, st.Account.Password
		query := `INSERT INTO account VALUES(1,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET
		name=excluded.name,algorithm=excluded.algorithm,memory=excluded.memory,iterations=excluded.iterations,
		parallelism=excluded.parallelism,salt=excluded.salt,hash=excluded.hash,recovery=excluded.recovery,
		epoch=excluded.epoch,theme=excluded.theme,profile=excluded.profile`
		_, err := tx.ExecContext(ctx, query, a.Name, v.Algorithm, v.Memory, v.Iterations, v.Parallelism,
			v.Salt, v.Hash, a.Recovery, a.Epoch, a.Theme, a.Profile)
		if err != nil {
			return err
		}
		if err = saveInventoryColumns(ctx, tx, a.Columns); err != nil {
			return err
		}
	}
	return persistAccessCatalog(ctx, tx, st)
}

func persistAccessCatalog(ctx context.Context, tx *sql.Tx, st access.State) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM grants"); err != nil {
		return err
	}
	for _, c := range st.Companies {
		if _, err := tx.ExecContext(ctx, "INSERT INTO companies(id,name) VALUES(?,?) "+
			"ON CONFLICT(id) DO UPDATE SET name=excluded.name", c.ID, c.Name); err != nil {
			return err
		}
	}
	for _, g := range st.Grants {
		if _, err := tx.ExecContext(ctx, "INSERT INTO grants(digest,epoch,expires) VALUES(?,?,?)",
			g.Digest, g.Epoch, g.Expires.UnixNano()); err != nil {
			return err
		}
	}
	next := int64(0)
	if !st.NextAttempt.IsZero() {
		next = st.NextAttempt.UnixNano()
	}
	_, err := tx.ExecContext(ctx, "UPDATE attempts SET failures=?,next_attempt=? WHERE id=1", st.Failures, next)
	return err
}
