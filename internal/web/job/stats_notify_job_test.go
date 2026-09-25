package job

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// fakeReporter records how many times SendReport was asked for.
type fakeReporter struct {
	calls int
}

func (f *fakeReporter) SendReport() { f.calls++ }

// The scheduled report must go out unconditionally.
//
// This is the regression test for a real report: "the bot works and manual backup
// works, but the automatic backup and report never come". The job refused to run
// unless Xray was up, so on a panel whose core had stopped the bot went silent —
// and because SendReport also drives the backup, so did the backups. Manual paths
// bypassed the job, which is why they kept working.
//
// A job that reports nothing fails here, which is the point: the assertion is on
// the call, not on any state the job might inspect.
func TestStatsNotifyJobReportsUnconditionally(t *testing.T) {
	reporter := &fakeReporter{}
	newStatsNotifyJobWithSender(reporter).Run()

	if reporter.calls != 1 {
		t.Fatalf("Run called SendReport %d times, want exactly 1 — the scheduled "+
			"report must not be conditioned on any server state", reporter.calls)
	}
}

// Run must be safe on a job that was not built through the constructor. The
// scheduler holds a value from NewStatsNotifyJob, but a malformed one should skip
// rather than dereference nil.
func TestStatsNotifyJobRunWithoutSender(t *testing.T) {
	(&StatsNotifyJob{}).Run() // must not panic
}

// funcIdents returns every identifier and string literal appearing inside a named
// function, taken from the AST.
//
// The AST is used rather than a text search over the file because the file's
// comment explains the removed guard by quoting it, and a text search cannot tell
// an explanation from the code it explains — a first version of this test failed
// on its own documentation. Comments are not part of the AST, so this cannot
// happen again.
func funcIdents(t *testing.T, filename, funcName string) (map[string]bool, map[string]bool) {
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
					lits[node.Value] = true
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

// A source guard for the same property as the behavioural test above.
//
// That test can only see what Run does through the sender it was handed.
// Re-introducing the guard by giving the job an Xray service again would compile
// and would only be caught here.
func TestStatsNotifyJobHasNoXrayDependency(t *testing.T) {
	idents, _ := funcIdents(t, "stats_notify_job.go", "Run")

	forbidden := map[string]string{
		"IsXrayRunning": "the core's running state must not gate the report",
		"DidXrayCrash":  "the core's crash state must not gate the report",
		"XrayService":   "the job must not hold an Xray service to query at all",
	}
	for name, why := range forbidden {
		if idents[name] {
			t.Errorf("stats_notify_job.go Run references %s: %s — this is the bug "+
				"that silenced the bot and the automatic backups", name, why)
		}
	}

	// And it must still do the one thing it exists for.
	if !idents["SendReport"] {
		t.Error("stats_notify_job.go Run no longer calls SendReport")
	}
}

// The job must keep depending on the narrow sender interface rather than on the
// concrete Telegram service, or the behavioural test above stops being possible.
func TestStatsNotifyJobDependsOnTheNarrowSender(t *testing.T) {
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, "stats_notify_job.go", nil, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	hasField := false
	ast.Inspect(parsed, func(n ast.Node) bool {
		field, ok := n.(*ast.Field)
		if !ok || field.Type == nil {
			return true
		}
		if id, ok := field.Type.(*ast.Ident); ok && id.Name == "reportSender" {
			hasField = true
		}
		return true
	})
	if !hasField {
		t.Error("StatsNotifyJob no longer holds a reportSender, so its behaviour " +
			"cannot be tested without a live bot")
	}
}
