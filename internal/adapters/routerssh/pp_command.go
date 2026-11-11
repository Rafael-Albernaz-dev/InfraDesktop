package routerssh

import (
	"context"
	"errors"
	"golang.org/x/crypto/ssh"
	"infradesktop/internal/adapters/routerssh/internal/routeros"
	"infradesktop/internal/modules/equipment"
	"log"
	"strings"
)

type ppCommandRunner struct {
	ctx      context.Context
	client   *ssh.Client
	budget   *outputBudget
	check    func() equipment.Code
	commands *int
}

func (r *ppCommandRunner) run(command string) (string, equipment.Code) {
	ctx, client, budget, check := r.ctx, r.client, r.budget, r.check
	if c := check(); c != equipment.OK {
		return "", c
	}
	session, code := openPPCommandSession(client)
	if code != equipment.OK {
		return "", code
	}
	defer session.Close()
	out, stderr := &limitedOutput{budget: budget}, &limitedOutput{budget: budget}
	session.Stdout = out
	session.Stderr = stderr
	budget.mu.Lock()
	(*r.commands)++
	budget.mu.Unlock()
	e := session.Run(command)
	logPPPingExecution(command, e, out, stderr)
	if budget.overflowed() {
		return "", equipment.Limit
	}
	if timedOut(ctx, e) {
		return "", equipment.RadioCodeDeadlineExceeded
	}
	text := out.buf.String()
	if routeros.CollectorSyntaxDiagnostic(text) || routeros.CollectorSyntaxDiagnostic(stderr.buf.String()) {
		return "", equipment.CollectorDefect
	}
	if permissionDiagnostic(text) || permissionDiagnostic(stderr.buf.String()) {
		return "", equipment.RadioCodeRemotePermissionDenied
	}
	if e != nil || stderr.buf.Len() != 0 {
		return "", equipment.Partial
	}
	return text, equipment.OK
}

func openPPCommandSession(client *ssh.Client) (*ssh.Session, equipment.Code) {
	session, e := client.NewSession()
	if e != nil {
		var denied *ssh.OpenChannelError
		if errors.As(e, &denied) && denied.Reason == ssh.Prohibited {
			return nil, equipment.RadioCodeRemotePermissionDenied
		}
		return nil, equipment.RadioCodeSSHConnectionLost
	}
	return session, equipment.OK
}

func (b *outputBudget) overflowed() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.overflow
}
func logPPPingExecution(command string, e error, out, stderr *limitedOutput) {

	if strings.Contains(command, "/tool ping address=") || strings.Contains(command, "/ping address=") {
		exit := -1
		var status *ssh.ExitError
		if e == nil {
			exit = 0
		} else if errors.As(e, &status) {
			exit = status.ExitStatus()
		}
		// Structural diagnostics only; no command, target, reply, credential
		// or remote error text is written to the log.
		log.Printf("routerssh ping execution: exit=%d stdout=%d stderr=%d esc=%d",
			exit,
			out.buf.Len(),
			stderr.buf.Len(),
			strings.Count(out.buf.String(),
				"\x1b"))
	}
}

func (r *ppCommandRunner) metrics() (int, int) {
	r.budget.mu.Lock()
	defer r.budget.mu.Unlock()
	return *r.commands, r.budget.n
}
func (r *ppCommandRunner) tighten(bytes int) {
	budget := r.budget
	budget.mu.Lock()
	defer budget.mu.Unlock()
	next := budget.n + bytes
	if budget.limit == 0 || next < budget.limit {
		budget.limit = next
	}
}
