package storage

import (
	"context"
	"infradesktop/internal/adapters/security"
	"infradesktop/internal/modules/access"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestProvisionCrashHelper(t *testing.T) {
	phase := os.Getenv("BF03_TEST_CRASH_PHASE")
	if phase == "" {
		return
	}
	st, e := Open(os.Getenv("BF03_TEST_STATE"))
	if e != nil {
		t.Fatal(e)
	}
	svc := access.New(st, security.Crypto{}, time.Now, 1000, "crash-test")
	cmd := access.Command{
		Op: "access.login", Name: "technical", Password: password,
		Owner: "helper", Remember: true,
	}
	v, c := svc.Handle(context.Background(), 1000, cmd)
	if c != access.OK {
		t.Fatal(c)
	}
	st.checkpoint = func(point string) {
		if point == phase {
			os.Exit(73)
		}
	}
	createCmd := access.Command{
		Op: "company.create", Name: "X", Operation: companyX,
		Owner: "helper", Session: v.Session, Profile: v.Profile, Activation: v.Activation,
	}
	svc.Handle(context.Background(), 1000, createCmd)
	t.Fatal("crash checkpoint was not reached")
}

func testProvisionCrashPhase(t *testing.T, phase string) {
	t.Helper()
	j := newJourney(t)
	dir := j.st.dir
	j.st.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestProvisionCrashHelper$")
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
}

func TestCompanyUnitActualCrashAndRestartAtProvisionBoundaries(t *testing.T) {
	phases := []string{"intent", "staged", "published", "ready", "before-catalog-commit", "catalog-committed"}
	for _, phase := range phases {
		t.Run(phase, func(t *testing.T) {
			testProvisionCrashPhase(t, phase)
		})
	}
}

func TestMigrationCrashHelper(t *testing.T) {
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

func initInterruptedMigrationDB(t *testing.T, dir string) {
	t.Helper()
	db, e := openPrivateDB(filepath.Join(dir, "infradesktop.db"), true)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
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
	stmt := "INSERT INTO account VALUES(1,?,?,?,?,?,?,?,?,?,?,?)"
	digest := crypto.Digest("fixture code")
	_, e = db.Exec(stmt, "technical", v.Algorithm, v.Memory, v.Iterations, v.Parallelism, v.Salt, v.Hash, digest, 9, "dark", companyX)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec("INSERT INTO companies(id,name) VALUES(?,?)", companyX, "X"); e != nil {
		t.Fatal(e)
	}
}

func testMigrationInterruptionPhase(t *testing.T, phase string) {
	t.Helper()
	dir := privateDir(t)
	initInterruptedMigrationDB(t, dir)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestMigrationCrashHelper$")
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
	if c != access.OK || state.Account.Epoch != 9 || state.Account.Theme != "dark" ||
		state.Account.Profile != companyX || len(state.Companies) != 1 || st.unitID != companyX {
		t.Fatal("interrupted migration lost context")
	}
}

func TestCompanyUnitMigrationResumesAfterActualInterruption(t *testing.T) {
	for _, phase := range []string{"global-migrated", "intent", "staged", "published", "ready"} {
		t.Run(phase, func(t *testing.T) {
			testMigrationInterruptionPhase(t, phase)
		})
	}
}
