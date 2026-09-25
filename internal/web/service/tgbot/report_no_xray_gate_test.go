package tgbot

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// funcFacts returns the identifiers and string literals used inside a named
// function, read from the AST.
//
// The AST is used rather than searching the file text because these functions
// carry comments explaining the old gate by quoting it; a text search cannot tell
// an explanation from the code it explains.
func funcFacts(t *testing.T, filename, funcName string) (map[string]bool, map[string]bool) {
	t.Helper()

	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, filename, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", filename, err)
	}

	idents := map[string]bool{}
	lits := map[string]bool{}
	found := false
	ast.Inspect(parsed, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Name == nil || fn.Name.Name != funcName || fn.Body == nil {
			return true
		}
		found = true
		ast.Inspect(fn.Body, func(inner ast.Node) bool {
			switch node := inner.(type) {
			case *ast.Ident:
				idents[node.Name] = true
			case *ast.SelectorExpr:
				if node.Sel != nil {
					idents[node.Sel.Name] = true
				}
			case *ast.BasicLit:
				if node.Kind == token.STRING {
					lits[strings.Trim(node.Value, "`\"")] = true
				}
			}
			return true
		})
		return false
	})
	if !found {
		t.Fatalf("%s not found in %s", funcName, filename)
	}
	return idents, lits
}

// The scheduled report must not be gated on the proxy core running.
//
// SendReport is what the Telegram-periodic job calls, and it is also what drives
// the automatic database backup. Gating either on Xray meant that a panel whose
// core had stopped sent no report and no backup — the opposite of what an operator
// needs, and the bug behind "the bot works and manual backup works, but the
// automatic backup and report never come".
//
// The guard is deliberately narrow: it checks SendReport and SendBackupToAdmins,
// not the whole file. prepareServerUsageInfo legitimately reads the process for
// the online-client list, with a nil check, and reporting the core's state is the
// point of the report. A file-wide ban would be wrong — only a gate that
// *suppresses* the send is.
func TestSendReportIsNotGatedOnTheProxyCore(t *testing.T) {
	const file = "tgbot_report.go"

	for _, fn := range []string{"SendReport", "SendBackupToAdmins"} {
		idents, _ := funcFacts(t, file, fn)
		for _, token := range []string{"IsXrayRunning", "DidXrayCrash"} {
			if idents[token] {
				t.Errorf("%s consults %s: a report or backup suppressed because the "+
					"proxy core is down is the failure this guard exists for. The core's "+
					"state belongs in the report body, not in a condition around it.",
					fn, token)
			}
		}
	}

	// SendReport must still reach both of its halves: the report body and the
	// backup call. Dropping either leaves the bot quietly incomplete.
	reportIdents, _ := funcFacts(t, file, "SendReport")
	for _, want := range []string{"sendServerUsage", "SendBackupToAdmins", "sendExhaustedToAdmins"} {
		if !reportIdents[want] {
			t.Errorf("SendReport no longer calls %s", want)
		}
	}
}

// The backup must not be gated on the bot reporting anything else either: it is a
// database dump, and an operator who asked for it should get it.
func TestBackupSendIsNotConditionalOnTheCore(t *testing.T) {
	idents, _ := funcFacts(t, "tgbot_report.go", "SendBackupToAdmins")

	if !idents["GetDb"] {
		t.Error("SendBackupToAdmins no longer reads the database")
	}
	if !idents["BackupFilename"] {
		t.Error("SendBackupToAdmins no longer names the backup file")
	}
	// It legitimately checks that the *bot* is running — that is about the bot,
	// not the proxy core.
	if !idents["IsRunning"] {
		t.Error("SendBackupToAdmins should still refuse when the bot itself is down")
	}
}

// When the core is not running, the report must carry its error text. Without it
// the operator reads "Status: error" and still has to go hunting for the reason,
// which is the situation the report exists to short-circuit.
func TestReportSurfacesTheCoreErrorWhenItIsDown(t *testing.T) {
	idents, lits := funcFacts(t, "tgbot_report.go", "prepareServerUsageInfo")

	if !idents["ErrorMsg"] {
		t.Error("prepareServerUsageInfo never reads Xray.ErrorMsg: the report will " +
			"show that the core is down without saying why")
	}
	if !lits["tgbot.messages.eventXrayCrashError"] {
		t.Error("prepareServerUsageInfo does not render the core error; reusing " +
			"tgbot.messages.eventXrayCrashError avoids adding a key to all locales")
	}
	if !idents["Running"] {
		t.Error("the core error must only be shown when the core is actually down, " +
			"otherwise a stale message from an earlier failure reads as current")
	}
	// The state must still be reported either way: a healthy core is worth saying.
	if !lits["tgbot.messages.xrayStatus"] {
		t.Error("prepareServerUsageInfo no longer reports the core's state")
	}
}
