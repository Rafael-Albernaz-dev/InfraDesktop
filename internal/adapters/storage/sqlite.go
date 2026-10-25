// Package storage owns durable state. No UI or account core opens SQLite.
package storage

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"infradesktop/internal/modules/access"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"sync"
	"time"
)

//go:embed migrations/*.sql
var migrations embed.FS

type Store struct {
	db                        *sql.DB
	lock                      *os.File
	mu                        sync.Mutex
	dir, installation, unitID string
	unit                      *sql.DB
	unitInfo                  os.FileInfo
	checkpoint                func(string)
}
type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func Open(dir string) (*Store, error) { return openStore(dir, nil) }

func openStore(dir string, checkpoint func(string)) (*Store, error) {
	if !filepath.IsAbs(dir) || filepath.Clean(dir) != dir {
		return nil, errors.New("invalid state path")
	}
	if e := ownedPrivate(dir, true); e != nil {
		return nil, e
	}
	lock, e := lockState(dir)
	if e != nil {
		return nil, e
	}
	transfer := false
	defer func() {
		if !transfer {
			lock.Close()
		}
	}()
	path := filepath.Join(dir, "infradesktop.db")
	if e := ownedPrivate(path, false); e != nil && !errors.Is(e, os.ErrNotExist) {
		return nil, e
	}
	_, statErr := os.Lstat(path)
	db, e := openPrivateDB(path, errors.Is(statErr, os.ErrNotExist))
	if e != nil {
		return nil, e
	}
	fail := func(err error) (*Store, error) { db.Close(); return nil, err }
	if _, e = db.Exec("PRAGMA journal_mode=WAL"); e != nil {
		return fail(e)
	}
	var check string
	if e = db.QueryRow("PRAGMA quick_check").Scan(&check); e != nil || check != "ok" {
		return fail(errors.New("invalid database"))
	}
	if e = backupBeforeSplit(db, dir); e != nil {
		return fail(e)
	}
	tx, e := db.Begin()
	if e != nil {
		return fail(e)
	}
	defer tx.Rollback()
	if _, e = tx.Exec("CREATE TABLE IF NOT EXISTS schema_migrations(version INTEGER PRIMARY KEY, checksum TEXT NOT NULL)"); e != nil {
		return fail(e)
	}
	if e = migrateGlobal(tx); e != nil {
		return fail(e)
	}
	if e = tx.Commit(); e != nil {
		return fail(e)
	}
	s := &Store{db: db, lock: lock, dir: dir, checkpoint: checkpoint}
	s.observe("global-migrated")
	if e = db.QueryRow("SELECT identity FROM installation WHERE id=1").Scan(&s.installation); e != nil {
		return fail(e)
	}
	if e = s.reconcile(); e != nil {
		return fail(e)
	}
	if _, c := s.Read(context.Background()); c != access.OK {
		return fail(errors.New("invalid account state"))
	}
	st, _ := s.Read(context.Background())
	if st.Account != nil && st.Account.Profile != "" {
		s.unit, e = s.openUnit(st.Account.Profile)
		if e != nil {
			return fail(e)
		}
		s.unitID = st.Account.Profile
		p, _ := s.unitPath(s.unitID)
		s.unitInfo, e = os.Lstat(p)
		if e != nil {
			s.unit.Close()
			return fail(e)
		}
	}
	transfer = true
	return s, nil
}
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.unit != nil {
		s.unit.Close()
	}
	e := s.db.Close()
	ce := s.lock.Close()
	if e != nil {
		return e
	}
	return ce
}
func load(ctx context.Context, q queryer) (access.State, error) {
	st := access.State{Companies: []access.Company{}}
	var a access.Account
	e := q.QueryRowContext(ctx, "SELECT name,algorithm,memory,iterations,parallelism,salt,hash,recovery,epoch,theme,profile FROM account WHERE id=1").Scan(&a.Name, &a.Password.Algorithm, &a.Password.Memory, &a.Password.Iterations, &a.Password.Parallelism, &a.Password.Salt, &a.Password.Hash, &a.Recovery, &a.Epoch, &a.Theme, &a.Profile)
	if e == nil {
		v := a.Password
		if v.Algorithm != "argon2id-v19" || v.Memory != 19456 || v.Iterations != 2 || v.Parallelism != 1 || len(v.Salt) != 16 || len(v.Hash) != 32 || len(a.Recovery) != 32 || a.Epoch == 0 || !access.ValidName(a.Name) || (a.Theme != "light" && a.Theme != "dark") {
			return st, errors.New("invalid account state")
		}
		st.Account = &a
	} else if !errors.Is(e, sql.ErrNoRows) {
		return st, e
	}
	var next int64
	if e = q.QueryRowContext(ctx, "SELECT failures,next_attempt FROM attempts WHERE id=1").Scan(&st.Failures, &next); e != nil {
		return st, e
	}
	if next > 0 {
		st.NextAttempt = time.Unix(0, next)
	}
	rows, e := q.QueryContext(ctx, "SELECT id,name FROM companies ORDER BY rowid")
	if e != nil {
		return st, e
	}
	for rows.Next() {
		var c access.Company
		if e = rows.Scan(&c.ID, &c.Name); e != nil {
			rows.Close()
			return st, e
		}
		if !access.ValidCompanyID(c.ID) || !access.ValidName(c.Name) {
			rows.Close()
			return st, errors.New("invalid catalog")
		}
		st.Companies = append(st.Companies, c)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return st, e
	}
	if st.Account != nil && st.Account.Profile != "" {
		found := false
		for _, c := range st.Companies {
			if c.ID == st.Account.Profile {
				found = true
			}
		}
		if !found {
			return st, errors.New("active company outside catalog")
		}
	}
	rows, e = q.QueryContext(ctx, "SELECT digest,epoch,expires FROM grants")
	if e != nil {
		return st, e
	}
	defer rows.Close()
	for rows.Next() {
		var g access.Grant
		var expiry int64
		if e = rows.Scan(&g.Digest, &g.Epoch, &expiry); e != nil {
			return st, e
		}
		g.Expires = time.Unix(0, expiry)
		st.Grants = append(st.Grants, g)
	}
	return st, rows.Err()
}
func (s *Store) Read(ctx context.Context) (access.State, access.Code) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.checkActive(); e != nil {
		return access.State{}, access.Unavailable
	}
	tx, e := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if e != nil {
		return access.State{}, access.Unavailable
	}
	defer tx.Rollback()
	st, e := load(ctx, tx)
	if e != nil {
		return access.State{}, access.Unavailable
	}
	if e = tx.Commit(); e != nil {
		return access.State{}, access.Unavailable
	}
	return st, access.OK
}
func (s *Store) Update(ctx context.Context, fn func(*access.State) access.Code) access.Code {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.checkActive(); e != nil {
		return access.Unavailable
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return access.Unavailable
	}
	defer tx.Rollback()
	st, e := load(ctx, tx)
	if e != nil {
		return access.Unavailable
	}
	if c := fn(&st); c != access.OK {
		return c
	}
	// Persist creation intent separately before touching the company file.
	var additions []access.Company
	for _, company := range st.Companies {
		var n int
		if e = tx.QueryRowContext(ctx, "SELECT count(*) FROM companies WHERE id=?", company.ID).Scan(&n); e != nil {
			return access.Unavailable
		}
		if n == 0 {
			additions = append(additions, company)
		}
	}
	if len(additions) > 0 {
		tx.Rollback()
		for _, company := range additions {
			if e = s.prepareIntent(ctx, company); e != nil {
				return access.Unavailable
			}
		}
		tx, e = s.db.BeginTx(ctx, nil)
		if e != nil {
			return access.Unavailable
		}
		defer tx.Rollback()
	}
	var candidate *sql.DB
	var candidateInfo os.FileInfo
	target := ""
	if st.Account != nil {
		target = st.Account.Profile
	}
	if target != s.unitID {
		if target != "" {
			p, err := s.unitPath(target)
			if err != nil {
				return access.Unavailable
			}
			candidateInfo, err = os.Lstat(p)
			if err != nil {
				return access.Unavailable
			}
			candidate, e = s.openUnit(target)
			if e != nil {
				return access.Unavailable
			}
		}
		defer func() {
			if candidate != nil {
				candidate.Close()
			}
		}()
	}
	if st.Account != nil {
		a := st.Account
		v := a.Password
		_, e = tx.ExecContext(ctx, `INSERT INTO account VALUES(1,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET name=excluded.name,algorithm=excluded.algorithm,memory=excluded.memory,iterations=excluded.iterations,parallelism=excluded.parallelism,salt=excluded.salt,hash=excluded.hash,recovery=excluded.recovery,epoch=excluded.epoch,theme=excluded.theme,profile=excluded.profile`, a.Name, v.Algorithm, v.Memory, v.Iterations, v.Parallelism, v.Salt, v.Hash, a.Recovery, a.Epoch, a.Theme, a.Profile)
		if e != nil {
			return access.Unavailable
		}
	}
	if _, e = tx.ExecContext(ctx, "DELETE FROM grants"); e != nil {
		return access.Unavailable
	}
	for _, c := range st.Companies {
		if _, e = tx.ExecContext(ctx, "INSERT INTO companies(id,name) VALUES(?,?) ON CONFLICT(id) DO UPDATE SET name=excluded.name", c.ID, c.Name); e != nil {
			return access.Unavailable
		}
	}
	for _, g := range st.Grants {
		if _, e = tx.ExecContext(ctx, "INSERT INTO grants(digest,epoch,expires) VALUES(?,?,?)", g.Digest, g.Epoch, g.Expires.UnixNano()); e != nil {
			return access.Unavailable
		}
	}
	next := int64(0)
	if !st.NextAttempt.IsZero() {
		next = st.NextAttempt.UnixNano()
	}
	if _, e = tx.ExecContext(ctx, "UPDATE attempts SET failures=?,next_attempt=? WHERE id=1", st.Failures, next); e != nil {
		return access.Unavailable
	}
	s.observe("before-catalog-commit")
	if e = tx.Commit(); e != nil {
		return access.Unavailable
	}
	s.observe("catalog-committed")
	if target != s.unitID {
		previous := s.unit
		s.unit, s.unitID = candidate, target
		s.unitInfo = nil
		if target != "" {
			s.unitInfo = candidateInfo
		}
		candidate = nil
		if previous != nil {
			previous.Close()
		}
	}
	return access.OK
}
func (s *Store) Diagnostics() (string, error) {
	var mode string
	var fk, busy int
	if e := s.db.QueryRow("PRAGMA journal_mode").Scan(&mode); e != nil {
		return "", e
	}
	if e := s.db.QueryRow("PRAGMA foreign_keys").Scan(&fk); e != nil {
		return "", e
	}
	if e := s.db.QueryRow("PRAGMA busy_timeout").Scan(&busy); e != nil {
		return "", e
	}
	return fmt.Sprintf("journal=%s foreign_keys=%d busy_timeout=%d pool=%d", mode, fk, busy, s.db.Stats().MaxOpenConnections), nil
}
