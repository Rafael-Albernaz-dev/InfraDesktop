package presentation

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAdministratorAuthorizationClassifiesSimulatedExecutor(t *testing.T) {
	for _, item := range []struct {
		exit int
		code string
	}{{0, "OK"}, {126, "ADMIN_AUTH_CANCELLED"}, {127, "ADMIN_AUTH_DENIED"}, {1, "ADMIN_AUTH_UNAVAILABLE"}} {
		t.Run(fmt.Sprint(item.exit), func(t *testing.T) {
			bin := t.TempDir()
			t.Setenv("PATH", bin)
			// This executable returns an exit status without running pkexec or its helper.
			body := fmt.Sprintf("#!/bin/sh\nexit %d\n", item.exit)
			if err := os.WriteFile(filepath.Join(bin, "pkexec"), []byte(body), 0700); err != nil {
				t.Fatal(err)
			}
			if code := runResetAuthorization(context.Background(), "synthetic", strings.Repeat("b", 64)); code != item.code {
				t.Fatalf("executor exit %d got %s want %s", item.exit, code, item.code)
			}
		})
	}
}
