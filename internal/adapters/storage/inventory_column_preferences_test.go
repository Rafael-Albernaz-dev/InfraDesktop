package storage

import (
	"context"
	"reflect"
	"testing"
	"time"

	"infradesktop/internal/adapters/security"
	"infradesktop/internal/modules/access"
)

func TestColumnPreferencesPersistOnAccountAndRejectInvalid(t *testing.T) {
	j := newJourney(t)
	j.create(companyX, "X", access.OK)
	j.create(companyY, "Y", access.OK)
	j.selectCompany(companyX, access.OK)
	q := j.q
	q.Op = "preferences.columns"
	columns := []string{"equipment", "block", "cpu"}
	q.Columns = &columns
	j.call(q, access.OK)
	for _, invalid := range [][]string{{"equipment", "cpu", "cpu"}, {"equipment", "shell"}, {"cpu"}, {}} {
		q.Columns = &invalid
		j.call(q, access.Invalid)
	}
	j.selectCompany(companyY, access.OK)
	q = j.q
	q.Op = "access.snapshot"
	if got := j.call(q, access.OK).Columns; !reflect.DeepEqual(got, []string{"equipment", "block", "cpu"}) {
		t.Fatal("company switch lost account preference", got)
	}
	dir := j.st.dir
	j.st.Close()
	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	j.st = store
	j.svc = access.New(store, security.Crypto{}, time.Now, 1000, "restart")
	got, code := j.svc.Handle(context.Background(), 1000,
		access.Command{Op: "access.login", Owner: "owner", Name: "technical", Password: password})
	if code != access.OK || !reflect.DeepEqual(got.Columns, []string{"equipment", "block", "cpu"}) {
		t.Fatal("restart lost account preference", code, got.Columns)
	}
}
