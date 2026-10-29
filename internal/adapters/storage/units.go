package storage

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"golang.org/x/sys/unix"
	"infradesktop/internal/modules/access"
	"net/url"
	"os"
	"path/filepath"
	"syscall"
)

//go:embed company_schema.sql
var companySchema string

//go:embed company_002_brand.sql
var companyBrandSchema string

//go:embed company_003_equipment.sql
var companyEquipmentSchema string

//go:embed company_004_operational.sql
var companyOperationalSchema string

//go:embed company_005_pppoe.sql
var companyPPPoESchema string

//go:embed company_006_pppoe_diagnostics.sql
var companyPPDiagnosticSchema string

//go:embed company_007_sessions_offline.sql
var companySessionsOfflineSchema string

//go:embed company_008_session_context.sql
var companySessionContextSchema string

//go:embed company_009_device_monitoring.sql
var companyDeviceMonitoringSchema string

//go:embed company_010_equipment_insights.sql
var companyEquipmentInsightsSchema string

//go:embed company_011_context_peaks.sql
var companyContextPeaksSchema string

func migrateGlobal(tx *sql.Tx) error {
	rows, e := tx.Query("SELECT version,checksum FROM schema_migrations ORDER BY version")
	if e != nil {
		return e
	}
	count := 0
	for rows.Next() {
		var v int
		var h string
		if e = rows.Scan(&v, &h); e != nil {
			rows.Close()
			return e
		}
		count++
		name := map[int]string{1: "001_access.sql", 2: "002_company_units.sql"}[v]
		b, err := migrations.ReadFile("migrations/" + name)
		if err != nil || v != count || h != checksum(b) {
			rows.Close()
			return errors.New("migration mismatch or unknown schema")
		}
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	if count == 0 {
		var n int
		if e = tx.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name!='schema_migrations'").Scan(&n); e != nil || n != 0 {
			return errors.New("unversioned database")
		}
	}
	for v := count + 1; v <= 2; v++ {
		name := map[int]string{1: "001_access.sql", 2: "002_company_units.sql"}[v]
		b, _ := migrations.ReadFile("migrations/" + name)
		if _, e = tx.Exec(string(b)); e != nil {
			return e
		}
		if _, e = tx.Exec("INSERT INTO schema_migrations VALUES(?,?)", v, checksum(b)); e != nil {
			return e
		}
	}
	return nil
}

func (s *Store) unitPath(id string) (string, error) {
	if !access.ValidCompanyID(id) {
		return "", errors.New("invalid company identity")
	}
	dir := filepath.Join(s.dir, "companies")
	if e := os.Mkdir(dir, 0700); e == nil {
		if e = syncDir(s.dir); e != nil {
			return "", e
		}
	} else if !errors.Is(e, os.ErrExist) {
		return "", e
	}
	if e := ownedPrivate(dir, true); e != nil {
		return "", e
	}
	return filepath.Join(dir, id+".db"), nil
}

func openPrivateDB(path string, create bool) (*sql.DB, error) {
	flags := syscall.O_RDWR | syscall.O_CLOEXEC | syscall.O_NOFOLLOW
	if create {
		flags |= syscall.O_CREAT | syscall.O_EXCL
	}
	fd, e := syscall.Open(path, flags, 0600)
	if e != nil {
		return nil, e
	}
	f := os.NewFile(uintptr(fd), path)
	info, e := f.Stat()
	f.Close()
	if e != nil {
		return nil, e
	}
	if e = ownedPrivate(path, false); e != nil {
		return nil, e
	}
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		if e = ownedPrivate(path+suffix, false); e != nil && !errors.Is(e, os.ErrNotExist) {
			return nil, e
		}
	}
	u := url.URL{Scheme: "file", Path: path}
	db, e := sql.Open("sqlite", u.String()+"?mode=rw&_pragma=foreign_keys(1)&_pragma=busy_timeout(1000)&_pragma=synchronous(FULL)")
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	fail := func(e error) (*sql.DB, error) { db.Close(); return nil, e }
	if e = db.Ping(); e != nil {
		return fail(e)
	}
	actual, e := os.Lstat(path)
	if e != nil || !os.SameFile(info, actual) {
		return fail(errors.New("database substituted"))
	}
	if e = ownedPrivate(path, false); e != nil {
		return fail(e)
	}
	return db, nil
}

func (s *Store) validateUnit(db *sql.DB, id string) error {
	var check string
	if e := db.QueryRow("PRAGMA quick_check").Scan(&check); e != nil || check != "ok" {
		return errors.New("invalid company database")
	}
	return s.validateUnitIdentity(db, id)
}
func (s *Store) validateUnitIdentity(db *sql.DB, id string) error {
	var n int
	var installation, company string
	if e := db.QueryRow("SELECT count(*) FROM schema_migrations").Scan(&n); e != nil {
		return e
	}
	if n < 1 || n > 11 {
		return errors.New("unknown company schema")
	}
	for v, src := range []string{companySchema, companyBrandSchema, companyEquipmentSchema, companyOperationalSchema, companyPPPoESchema, companyPPDiagnosticSchema, companySessionsOfflineSchema, companySessionContextSchema, companyDeviceMonitoringSchema, companyEquipmentInsightsSchema, companyContextPeaksSchema} {
		if v+1 > n {
			break
		}
		var h string
		if e := db.QueryRow("SELECT checksum FROM schema_migrations WHERE version=?", v+1).Scan(&h); e != nil || h != checksum([]byte(src)) {
			return errors.New("company migration mismatch")
		}
	}
	schemaCount := n
	if e := db.QueryRow("SELECT count(*) FROM identity").Scan(&n); e != nil || n != 1 {
		return errors.New("invalid company identity")
	}
	if e := db.QueryRow("SELECT installation,company FROM identity WHERE singleton=1").Scan(&installation, &company); e != nil || installation != s.installation || company != id {
		return errors.New("company identity mismatch")
	}
	for version := schemaCount + 1; version <= 11; version++ {
		src := map[int]string{2: companyBrandSchema, 3: companyEquipmentSchema, 4: companyOperationalSchema, 5: companyPPPoESchema, 6: companyPPDiagnosticSchema, 7: companySessionsOfflineSchema, 8: companySessionContextSchema, 9: companyDeviceMonitoringSchema, 10: companyEquipmentInsightsSchema, 11: companyContextPeaksSchema}[version]
		tx, e := db.Begin()
		if e != nil {
			return e
		}
		if _, e = tx.Exec(src); e == nil && version == 7 {
			e = migrateReadReceipts(tx)
		}
		if e == nil {
			_, e = tx.Exec("INSERT INTO schema_migrations VALUES(?,?)", version, checksum([]byte(src)))
		}
		if e != nil {
			tx.Rollback()
			return e
		}
		if e = tx.Commit(); e != nil {
			return e
		}
	}
	return nil
}

func (s *Store) openUnit(id string) (*sql.DB, error) {
	path, e := s.unitPath(id)
	if e != nil {
		return nil, e
	}
	db, e := openPrivateDB(path, false)
	if e != nil {
		return nil, e
	}
	if e = s.validateUnit(db, id); e != nil {
		db.Close()
		return nil, e
	}
	if _, e = db.Exec("PRAGMA journal_mode=WAL"); e != nil {
		db.Close()
		return nil, e
	}
	return db, nil
}

func syncFile(path string) error {
	f, e := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if e != nil {
		return e
	}
	defer f.Close()
	return f.Sync()
}
func syncDir(path string) error { return syncFile(path) }

func (s *Store) provision(id string, ready bool) error {
	path, e := s.unitPath(id)
	if e != nil {
		return e
	}
	if _, e = os.Lstat(path); e == nil {
		db, e := s.openUnit(id)
		if e != nil {
			return e
		}
		return db.Close()
	} else if !errors.Is(e, os.ErrNotExist) {
		return e
	}
	if ready {
		return errors.New("ready company database missing")
	}
	pending := path + ".pending"
	var db *sql.DB
	stage := ""
	if _, err := os.Lstat(pending); err == nil {
		db, e = openPrivateDB(pending, false)
		if e == nil {
			e = s.validateUnit(db, id)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	} else {
		// An interrupted uninitialized build has a unique name and is never adopted
		// or overwritten. Only the validated staged unit receives the stable name.
		f, err := os.CreateTemp(filepath.Dir(path), id+".building-")
		if err != nil {
			return err
		}
		stage = f.Name()
		if err = f.Close(); err != nil {
			return err
		}
		db, e = openPrivateDB(stage, false)
		if e == nil {
			var tx *sql.Tx
			tx, e = db.Begin()
			if e == nil {
				_, e = tx.Exec("CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY,checksum TEXT NOT NULL);" + companySchema)
				if e == nil {
					_, e = tx.Exec("INSERT INTO schema_migrations VALUES(1,?)", checksum([]byte(companySchema)))
				}
				if e == nil {
					_, e = tx.Exec("INSERT INTO identity VALUES(1,?,?)", s.installation, id)
				}
				if e == nil {
					_, e = tx.Exec(companyBrandSchema)
				}
				if e == nil {
					_, e = tx.Exec("INSERT INTO schema_migrations VALUES(2,?)", checksum([]byte(companyBrandSchema)))
				}
				if e == nil {
					e = tx.Commit()
				} else {
					tx.Rollback()
				}
			}
		}
	}
	if db != nil {
		ce := db.Close()
		if e == nil {
			e = ce
		}
	}
	if e != nil {
		return e
	}
	if stage != "" {
		if e = syncFile(stage); e != nil {
			return e
		}
		if e = unix.Renameat2(unix.AT_FDCWD, stage, unix.AT_FDCWD, pending, unix.RENAME_NOREPLACE); e != nil {
			return e
		}
	}
	s.observe("staged")
	if e = syncFile(pending); e != nil {
		return e
	}
	// Atomic no-replace publication; canonical path is never overwritten.
	if e = unix.Renameat2(unix.AT_FDCWD, pending, unix.AT_FDCWD, path, unix.RENAME_NOREPLACE); e != nil {
		return e
	}
	s.observe("published")
	return syncDir(filepath.Dir(path))
}

func (s *Store) prepareIntent(ctx context.Context, c access.Company) error {
	if !access.ValidCompanyID(c.ID) {
		return errors.New("invalid company identity")
	}
	var count, exists int
	if e := s.db.QueryRowContext(ctx, "SELECT count(*),count(CASE WHEN id=? THEN 1 END) FROM unit_intents", c.ID).Scan(&count, &exists); e != nil {
		return e
	}
	if count >= 100 && exists == 0 {
		return errors.New("company capacity reached")
	}
	if _, e := s.db.ExecContext(ctx, "INSERT INTO unit_intents(id,name) VALUES(?,?) ON CONFLICT(id) DO NOTHING", c.ID, c.Name); e != nil {
		return e
	}
	var name string
	var ready bool
	if e := s.db.QueryRowContext(ctx, "SELECT name,ready FROM unit_intents WHERE id=?", c.ID).Scan(&name, &ready); e != nil {
		return e
	}
	s.observe("intent")
	if name != c.Name {
		return errors.New("operation identity reused with other name")
	}
	if e := s.provision(c.ID, ready); e != nil {
		return e
	}
	_, e := s.db.ExecContext(ctx, "UPDATE unit_intents SET ready=1 WHERE id=?", c.ID)
	if e == nil {
		s.observe("ready")
	}
	return e
}
func (s *Store) reconcile() error {
	artifacts, e := filepath.Glob(filepath.Join(s.dir, "companies", "*.building-*"))
	if e != nil {
		return e
	}
	if len(artifacts) > 0 {
		fmt.Fprintf(os.Stderr, "company provisioning: preserved %d unclaimed staging artifacts; no unknown file removed\n", len(artifacts))
	}
	rows, e := s.db.Query("SELECT id,name FROM unit_intents ORDER BY rowid")
	if e != nil {
		return e
	}
	var companies []access.Company
	for rows.Next() {
		var c access.Company
		if e = rows.Scan(&c.ID, &c.Name); e != nil {
			rows.Close()
			return e
		}
		companies = append(companies, c)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, c := range companies {
		if e = s.prepareIntent(context.Background(), c); e != nil {
			return fmt.Errorf("company provisioning unavailable: %w", e)
		}
		if _, e = s.db.Exec("INSERT INTO companies(id,name) VALUES(?,?) ON CONFLICT(id) DO NOTHING", c.ID, c.Name); e != nil {
			return e
		}
	}
	return nil
}

// SQLite itself produces a coherent snapshot, including committed WAL frames.
func backupBeforeSplit(db *sql.DB, dir string) error {
	var count int
	if e := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE name='schema_migrations'").Scan(&count); e != nil {
		return e
	}
	if count == 0 {
		return nil
	}
	if e := db.QueryRow("SELECT count(*) FROM schema_migrations").Scan(&count); e != nil {
		return e
	}
	if count != 1 {
		return nil
	}
	var v int
	var h string
	b, _ := migrations.ReadFile("migrations/001_access.sql")
	if e := db.QueryRow("SELECT version,checksum FROM schema_migrations").Scan(&v, &h); e != nil {
		return e
	}
	if v != 1 || h != checksum(b) {
		return errors.New("migration mismatch")
	}
	f, e := os.CreateTemp(dir, "pre-bf03-*.db")
	if e != nil {
		return e
	}
	path := f.Name()
	if e = f.Close(); e != nil {
		return e
	}
	if _, e = db.Exec("VACUUM INTO ?", path); e != nil {
		return e
	}
	if e = syncFile(path); e != nil {
		return e
	}
	return syncDir(dir)
}

// A replaced/unlinked active path never silently redirects an existing context.
func (s *Store) checkActive() error {
	if s.unit == nil {
		return nil
	}
	path, e := s.unitPath(s.unitID)
	if e != nil {
		return e
	}
	if e = ownedPrivate(path, false); e != nil {
		return e
	}
	current, e := os.Lstat(path)
	if e != nil {
		return e
	}
	if s.unitInfo == nil || !os.SameFile(s.unitInfo, current) {
		return errors.New("active company substituted")
	}
	return nil
}

// Internal crash observation is injected only by adapter tests; no environment,
// IPC, build flag or production configuration can enable it.
func (s *Store) observe(point string) {
	if s.checkpoint != nil {
		s.checkpoint(point)
	}
}
