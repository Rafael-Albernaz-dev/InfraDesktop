package storage

import (
	"context"
	"database/sql"
	"fmt"
	"infradesktop/internal/adapters/security"
	"infradesktop/internal/modules/access"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const companyX = "aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa"
const companyY = "bbbbbbbb-bbbb-4bbb-bbbb-bbbbbbbbbbbb"

type journey struct {
	t        *testing.T
	st       *Store
	svc      *access.Service
	now      time.Time
	q        access.Command
	recovery string
}

func newJourney(t *testing.T) *journey {
	t.Helper()
	st, e := Open(privateDir(t))
	if e != nil {
		t.Fatal(e)
	}
	j := &journey{t: t, st: st, now: time.Now()}
	j.svc = access.New(st, security.Crypto{}, func() time.Time { return j.now }, 1000, "start", st)
	t.Cleanup(func() { j.st.Close() })
	j.recovery = j.call(access.Command{Op: "access.register", Name: "technical", Password: password}, access.OK).Recovery
	j.now = j.now.Add(31 * time.Second)
	v := j.call(access.Command{Op: "access.login", Owner: "owner", Name: "technical", Password: password, Remember: true}, access.OK)
	j.q = access.Command{Owner: "owner", Session: v.Session, Activation: v.Activation, Profile: v.Profile}
	return j
}
func (j *journey) call(q access.Command, want access.Code) access.View {
	j.t.Helper()
	v, c := j.svc.Handle(context.Background(), 1000, q)
	if c != want {
		j.t.Fatalf("%s: got %s want %s", q.Op, c, want)
	}
	return v
}
func (j *journey) create(id, name string, want access.Code) access.View {
	j.t.Helper()
	q := j.q
	q.Op = "company.create"
	q.Operation = id
	q.Name = name
	return j.call(q, want)
}
func (j *journey) selectCompany(id string, want access.Code) access.View {
	j.t.Helper()
	q := j.q
	q.Op = "company.select"
	q.Name = id
	v := j.call(q, want)
	if want == access.OK {
		j.q.Profile = v.Profile
		j.q.Activation = v.Activation
	}
	return v
}

func TestBF03SeparateUnitsAndBoundedSwitch(t *testing.T) {
	j := newJourney(t)
	j.create(companyX, "Same name", access.OK)
	j.create(companyY, "Same name", access.OK)
	v := j.create(companyX, "Same name", access.OK)
	if len(v.Companies) != 2 {
		t.Fatal("retry duplicated company")
	}
	j.create(companyX, "changed payload", access.Invalid)
	var old *sql.DB
	for i, id := range []string{companyX, companyY} {
		j.selectCompany(id, access.OK)
		if old != nil {
			if _, e := old.Exec("SELECT 1"); e == nil {
				t.Fatal("old handle remained usable")
			}
		}
		if _, e := j.st.unit.Exec("CREATE TABLE fixture(id TEXT PRIMARY KEY,value TEXT); INSERT INTO fixture VALUES('same-local-id',?)", fmt.Sprint(i)); e != nil {
			t.Fatal(e)
		}
		old = j.st.unit
	}
	for i := 0; i < 120; i++ {
		id := companyX
		want := "0"
		if i%2 != 0 {
			id = companyY
			want = "1"
		}
		stale := j.q
		j.selectCompany(id, access.OK)
		stale.Op = "access.snapshot"
		j.call(stale, access.Changed)
		var got string
		if e := j.st.unit.QueryRow("SELECT value FROM fixture WHERE id='same-local-id'").Scan(&got); e != nil || got != want {
			t.Fatal("cross-company data", e)
		}
		if j.st.db.Stats().OpenConnections != 1 || j.st.unit.Stats().OpenConnections != 1 {
			t.Fatal("unbounded pool")
		}
	}
	j.st.Close()
	s, e := Open(j.st.dir)
	if e != nil {
		t.Fatal(e)
	}
	j.st = s
	if s.unitID != companyY {
		t.Fatal("active context lost on restart")
	}
	var got string
	s.unit.QueryRow("SELECT value FROM fixture").Scan(&got)
	if got != "1" {
		t.Fatal("wrong reopened unit")
	}
}

func TestBF03SelectionFailsClosed(t *testing.T) {
	for _, kind := range []string{"missing", "corrupt", "symlink", "permissions", "foreign-company", "foreign-installation", "future-schema", "traversal", "commit"} {
		t.Run(kind, func(t *testing.T) {
			j := newJourney(t)
			j.create(companyX, "X", access.OK)
			j.create(companyY, "Y", access.OK)
			j.selectCompany(companyX, access.OK)
			target, _ := j.st.unitPath(companyY)
			before := j.q
			handle := j.st.unit
			switch kind {
			case "missing":
				os.Remove(target)
			case "corrupt":
				os.WriteFile(target, []byte("preserve corrupt data"), 0600)
			case "symlink":
				os.Remove(target)
				os.Symlink(filepath.Join(j.st.dir, "infradesktop.db"), target)
			case "permissions":
				os.Chmod(target, 0644)
			case "traversal":
				j.selectCompany("../"+companyY, access.Invalid)
				return
			case "commit":
				j.st.db.Exec("CREATE TRIGGER fail_select BEFORE UPDATE ON account BEGIN SELECT RAISE(FAIL,'injected commit path'); END")
			default:
				db, e := openPrivateDB(target, false)
				if e != nil {
					t.Fatal(e)
				}
				switch kind {
				case "foreign-company":
					_, e = db.Exec("UPDATE identity SET company=?", companyX)
				case "foreign-installation":
					_, e = db.Exec("UPDATE identity SET installation=?", strings.Repeat("0", 64))
				case "future-schema":
					_, e = db.Exec("INSERT INTO schema_migrations VALUES(3,'future')")
				}
				db.Close()
				if e != nil {
					t.Fatal(e)
				}
			}
			j.selectCompany(companyY, access.Unavailable)
			if j.q != before || j.st.unit != handle || j.st.unitID != companyX {
				t.Fatal("partial context switch")
			}
			var profile string
			j.st.db.QueryRow("SELECT profile FROM account").Scan(&profile)
			if profile != companyX {
				t.Fatal("catalog changed on failure")
			}
		})
	}
}

func TestBF03IntentRecoveryAndCommitFailure(t *testing.T) {
	j := newJourney(t)
	if _, e := j.st.db.Exec("CREATE TRIGGER fail_catalog BEFORE INSERT ON companies BEGIN SELECT RAISE(FAIL,'injected catalog failure'); END"); e != nil {
		t.Fatal(e)
	}
	j.create(companyX, "X", access.Unavailable)
	var n int
	j.st.db.QueryRow("SELECT count(*) FROM companies").Scan(&n)
	if n != 0 {
		t.Fatal("premature catalog publication")
	}
	var ready bool
	j.st.db.QueryRow("SELECT ready FROM unit_intents WHERE id=?", companyX).Scan(&ready)
	if !ready {
		t.Fatal("intent not durable")
	}
	j.st.db.Exec("DROP TRIGGER fail_catalog")
	j.st.Close()
	s, e := Open(j.st.dir)
	if e != nil {
		t.Fatal(e)
	}
	j.st = s
	st, c := s.Read(context.Background())
	if c != access.OK || len(st.Companies) != 1 || st.Companies[0].ID != companyX {
		t.Fatal("intent not reconciled")
	}
	// A staging artifact without a validated identity is never overwritten/deleted.
	if _, e = s.db.Exec("INSERT INTO unit_intents(id,name) VALUES(?,?)", companyY, "Y"); e != nil {
		t.Fatal(e)
	}
	p, _ := s.unitPath(companyY)
	raw := []byte("unknown artifact")
	os.WriteFile(p+".pending", raw, 0600)
	s.Close()
	if failed, e := Open(s.dir); e == nil {
		failed.Close()
		t.Fatal("unknown artifact adopted")
	}
	actual, _ := os.ReadFile(p + ".pending")
	if string(actual) != string(raw) {
		t.Fatal("unknown artifact removed")
	}
}

func TestBF03MigrationFromOneAndCoherentBackup(t *testing.T) {
	dir := privateDir(t)
	path := filepath.Join(dir, "infradesktop.db")
	db, e := openPrivateDB(path, true)
	if e != nil {
		t.Fatal(e)
	}
	b, _ := migrations.ReadFile("migrations/001_access.sql")
	_, e = db.Exec("PRAGMA journal_mode=WAL; CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY,checksum TEXT NOT NULL);" + string(b))
	if e != nil {
		t.Fatal(e)
	}
	db.Exec("INSERT INTO schema_migrations VALUES(1,?)", checksum(b))
	crypto := security.Crypto{}
	v, c := crypto.Hash(password)
	if c != access.OK {
		t.Fatal(c)
	}
	_, e = db.Exec("INSERT INTO account VALUES(1,?,?,?,?,?,?,?,?,?,?,?)", "technical", v.Algorithm, v.Memory, v.Iterations, v.Parallelism, v.Salt, v.Hash, crypto.Digest("old-recovery"), 7, "dark", companyY)
	if e != nil {
		t.Fatal(e)
	}
	db.Exec("INSERT INTO companies(id,name) VALUES(?,?),(?,?)", companyX, "X", companyY, "Y")
	grant := strings.Repeat("a", 64)
	db.Exec("INSERT INTO grants(digest,epoch,expires) VALUES(?,7,?)", crypto.Digest(grant), time.Now().Add(time.Hour).UnixNano())
	// Keep a WAL reader to prove backup uses SQLite, not just a copied main file.
	reader, _ := db.Begin()
	var held int
	if e = reader.QueryRow("SELECT count(*) FROM companies").Scan(&held); e != nil {
		t.Fatal(e)
	}
	defer reader.Rollback()
	// Storage lock is the ownership prerequisite; fixture connection closes before Open.
	s, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	reader.Rollback()
	db.Close()
	st, c := s.Read(context.Background())
	if c != access.OK || st.Account.Epoch != 7 || st.Account.Theme != "dark" || st.Account.Profile != companyY || len(st.Companies) != 2 || st.Companies[0].ID != companyX || len(st.Grants) != 1 || !crypto.Verify(password, st.Account.Password) {
		t.Fatal("migration lost state")
	}
	svc := access.New(s, crypto, time.Now, 1000, "new")
	view, c := svc.Handle(context.Background(), 1000, access.Command{Op: "access.resume", Owner: "fixture", Grant: grant})
	if c != access.OK || view.Profile != companyY {
		t.Fatal("valid grant lost")
	}
	backups, _ := filepath.Glob(filepath.Join(dir, "pre-bf03-*.db"))
	if len(backups) != 1 {
		t.Fatal("backup missing")
	}
	backup, e := openPrivateDB(backups[0], false)
	if e != nil {
		t.Fatal(e)
	}
	defer backup.Close()
	var version int
	backup.QueryRow("SELECT max(version) FROM schema_migrations").Scan(&version)
	if version != 1 {
		t.Fatal("backup not old schema")
	}
	var count int
	backup.QueryRow("SELECT count(*) FROM companies").Scan(&count)
	if count != 2 {
		t.Fatal("backup incomplete")
	}
}

func TestBF03CrashHelper(t *testing.T) {
	phase := os.Getenv("BF03_TEST_CRASH_PHASE")
	if phase == "" {
		return
	}
	st, e := Open(os.Getenv("BF03_TEST_STATE"))
	if e != nil {
		t.Fatal(e)
	}
	svc := access.New(st, security.Crypto{}, time.Now, 1000, "crash-test")
	v, c := svc.Handle(context.Background(), 1000, access.Command{Op: "access.login", Name: "technical", Password: password, Owner: "helper", Remember: true})
	if c != access.OK {
		t.Fatal(c)
	}
	st.checkpoint = func(point string) {
		if point == phase {
			os.Exit(73)
		}
	}
	svc.Handle(context.Background(), 1000, access.Command{Op: "company.create", Name: "X", Operation: companyX, Owner: "helper", Session: v.Session, Profile: v.Profile, Activation: v.Activation})
	t.Fatal("crash checkpoint was not reached")
}
func TestBF03ActualCrashAndRestartAtProvisionBoundaries(t *testing.T) {
	for _, phase := range []string{"intent", "staged", "published", "ready", "before-catalog-commit", "catalog-committed"} {
		t.Run(phase, func(t *testing.T) {
			j := newJourney(t)
			dir := j.st.dir
			j.st.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestBF03CrashHelper$")
			cmd.Env = append(os.Environ(), "BF03_TEST_CRASH_PHASE="+phase, "BF03_TEST_STATE="+dir)
			output, e := cmd.CombinedOutput()
			if exit, ok := e.(*exec.ExitError); !ok || exit.ExitCode() != 73 {
				t.Fatalf("crash injection did not exit 73: %v %s", e, output)
			}
			s, e := Open(dir)
			if e != nil {
				t.Fatal(e)
			}
			j.st = s
			st, c := s.Read(context.Background())
			if c != access.OK || len(st.Companies) != 1 || st.Companies[0].ID != companyX {
				t.Fatal("restart did not reconcile intent")
			}
			db, e := s.openUnit(companyX)
			if e != nil {
				t.Fatal(e)
			}
			db.Close()
		})
	}
}
func TestBF03SubstitutedActiveHandleRejected(t *testing.T) {
	j := newJourney(t)
	j.create(companyX, "X", access.OK)
	j.selectCompany(companyX, access.OK)
	p, _ := j.st.unitPath(companyX)
	if e := os.Rename(p, p+".moved"); e != nil {
		t.Fatal(e)
	}
	// Even a byte-identical replacement cannot redirect the existing active lease.
	data, e := os.ReadFile(p + ".moved")
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(p, data, 0600); e != nil {
		t.Fatal(e)
	}
	q := j.q
	q.Op = "access.snapshot"
	j.call(q, access.Unavailable)
}

func TestBF03LateTransactionRemainsBoundToOriginalUnit(t *testing.T) {
	j := newJourney(t)
	j.create(companyX, "X", access.OK)
	j.create(companyY, "Y", access.OK)
	for _, id := range []string{companyX, companyY} {
		j.selectCompany(id, access.OK)
		if _, e := j.st.unit.Exec("CREATE TABLE fixture(id TEXT PRIMARY KEY,value TEXT);INSERT INTO fixture VALUES('same','original')"); e != nil {
			t.Fatal(e)
		}
	}
	j.selectCompany(companyX, access.OK)
	original := j.st.unit
	tx, e := original.Begin()
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback()
	if _, e = tx.Exec("UPDATE fixture SET value='late X'"); e != nil {
		t.Fatal(e)
	}
	j.selectCompany(companyY, access.OK)
	if e = tx.Commit(); e != nil {
		t.Fatal(e)
	}
	var got string
	j.st.unit.QueryRow("SELECT value FROM fixture").Scan(&got)
	if got != "original" {
		t.Fatal("late X transaction contaminated Y")
	}
	j.selectCompany(companyX, access.OK)
	j.st.unit.QueryRow("SELECT value FROM fixture").Scan(&got)
	if got != "late X" {
		t.Fatal("late transaction lost original identity")
	}
	if _, e = original.Exec("SELECT 1"); e == nil {
		t.Fatal("new operation used released handle")
	}
}

func TestBF03MigrationCrashHelper(t *testing.T) {
	phase := os.Getenv("BF03_TEST_MIGRATION_PHASE")
	if phase == "" {
		return
	}
	s, e := openStore(os.Getenv("BF03_TEST_STATE"), func(point string) {
		if point == phase {
			os.Exit(74)
		}
	})
	if e != nil {
		t.Fatal(e)
	}
	s.Close()
	t.Fatal("migration crash checkpoint not reached")
}
func TestBF03MigrationResumesAfterActualInterruption(t *testing.T) {
	for _, phase := range []string{"global-migrated", "intent", "staged", "published", "ready"} {
		t.Run(phase, func(t *testing.T) {
			dir := privateDir(t)
			db, e := openPrivateDB(filepath.Join(dir, "infradesktop.db"), true)
			if e != nil {
				t.Fatal(e)
			}
			b, _ := migrations.ReadFile("migrations/001_access.sql")
			if _, e = db.Exec("CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY,checksum TEXT NOT NULL);" + string(b)); e != nil {
				t.Fatal(e)
			}
			db.Exec("INSERT INTO schema_migrations VALUES(1,?)", checksum(b))
			crypto := security.Crypto{}
			v, c := crypto.Hash(password)
			if c != access.OK {
				t.Fatal(c)
			}
			_, e = db.Exec("INSERT INTO account VALUES(1,?,?,?,?,?,?,?,?,?,?,?)", "technical", v.Algorithm, v.Memory, v.Iterations, v.Parallelism, v.Salt, v.Hash, crypto.Digest("fixture code"), 9, "dark", companyX)
			if e != nil {
				t.Fatal(e)
			}
			if _, e = db.Exec("INSERT INTO companies(id,name) VALUES(?,?)", companyX, "X"); e != nil {
				t.Fatal(e)
			}
			db.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestBF03MigrationCrashHelper$")
			cmd.Env = append(os.Environ(), "BF03_TEST_MIGRATION_PHASE="+phase, "BF03_TEST_STATE="+dir)
			output, e := cmd.CombinedOutput()
			if exit, ok := e.(*exec.ExitError); !ok || exit.ExitCode() != 74 {
				t.Fatalf("migration crash injection: %v %s", e, output)
			}
			st, e := Open(dir)
			if e != nil {
				t.Fatal(e)
			}
			defer st.Close()
			state, c := st.Read(context.Background())
			if c != access.OK || state.Account.Epoch != 9 || state.Account.Theme != "dark" || state.Account.Profile != companyX || len(state.Companies) != 1 || st.unitID != companyX {
				t.Fatal("interrupted migration lost context")
			}
		})
	}
}
