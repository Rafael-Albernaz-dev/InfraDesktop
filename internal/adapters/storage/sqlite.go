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
)

//go:embed migrations/*.sql migrations/company/*.sql
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
	if s.checkActive() != nil {
		return access.Unavailable
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return access.Unavailable
	}
	defer tx.Rollback()
	st, err := load(ctx, tx)
	if err != nil {
		return access.Unavailable
	}
	if code := fn(&st); code != access.OK {
		return code
	}
	additions, err := catalogAdditions(ctx, tx, st.Companies)
	if err != nil {
		return access.Unavailable
	}
	tx, err = s.prepareCatalogAdditions(ctx, tx, additions)
	if err != nil {
		return access.Unavailable
	}
	defer tx.Rollback()
	candidate, info, target, err := s.catalogCandidate(st)
	if err != nil {
		return access.Unavailable
	}
	defer func() {
		if candidate != nil {
			candidate.Close()
		}
	}()
	if err = persistAccessState(ctx, tx, st); err != nil {
		return access.Unavailable
	}
	s.observe("before-catalog-commit")
	if err = tx.Commit(); err != nil {
		return access.Unavailable
	}
	s.observe("catalog-committed")
	if target != s.unitID {
		previous := s.unit
		s.unit, s.unitID, s.unitInfo = candidate, target, info
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
