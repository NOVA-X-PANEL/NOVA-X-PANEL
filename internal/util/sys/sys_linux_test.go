//go:build linux

package sys

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// countConnections was rewritten from a line scanner to a byte counter for
// speed. These cases pin the result so a faster implementation cannot quietly
// change the number: the status page reports it as the connection count.
func TestCountConnections(t *testing.T) {
	header := "  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode\n"
	row := func(n int) string {
		return "   " + strconv.Itoa(n) + ": 0100007F:1F90 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 1" + strconv.Itoa(n) + " 1 0000000000000000 100 0 0 10 0\n"
	}

	cases := []struct {
		name    string
		content string
		want    int
	}{
		{"header only", header, 0},
		{"header and three rows", header + row(1) + row(2) + row(3), 3},
		{"empty file", "", 0},
		// bufio.Scanner emits a final line with no trailing newline, so the byte
		// counter must count it too or the number would drop by one.
		{"no trailing newline on the last row", header + row(1) + row(2) + strings.TrimSuffix(row(3), "\n"), 3},
		{"no trailing newline, header only", strings.TrimSuffix(header, "\n"), 0},
		{"a single row and nothing else", row(1), 0}, // the only line is treated as the header
		{"blank lines are still lines", header + "\n" + row(1), 2},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "net")
			if err := os.WriteFile(path, []byte(tc.content), 0o600); err != nil {
				t.Fatalf("write fixture: %v", err)
			}
			got, err := countConnections(path)
			if err != nil {
				t.Fatalf("countConnections: %v", err)
			}
			if got != tc.want {
				t.Errorf("countConnections = %d, want %d", got, tc.want)
			}
		})
	}
}

// A missing file is not an error: /proc/net/tcp6 is absent when IPv6 is off, and
// the status page should show zero rather than a warning on every refresh.
func TestCountConnectionsMissingFile(t *testing.T) {
	got, err := countConnections(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("missing file should not error, got %v", err)
	}
	if got != 0 {
		t.Errorf("missing file should count 0, got %d", got)
	}
}

// The connection count must survive a file much larger than the scratch buffer,
// since a busy server's /proc/net/tcp runs to thousands of lines. This also
// guards the loop boundary: a chunk that ends exactly on a newline must not
// double-count the following line.
func TestCountConnectionsLargeFile(t *testing.T) {
	var b strings.Builder
	b.WriteString("header\n")
	const rows = 20000 // far beyond a single 32 KiB read
	for i := 0; i < rows; i++ {
		b.WriteString("row-")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("\n")
	}
	path := filepath.Join(t.TempDir(), "net")
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	got, err := countConnections(path)
	if err != nil {
		t.Fatalf("countConnections: %v", err)
	}
	if got != rows {
		t.Errorf("countConnections = %d, want %d", got, rows)
	}
}

// The byte counter must stay cheaper on memory than the line scanner it
// replaced. This measures bytes allocated per call rather than allocation count,
// because that is what actually differs: bufio.Scanner cost ~4,250 B per call on
// a 5,000-entry file (4 allocations) while counting bytes costs ~150 B (3
// allocations). Allocation count alone cannot tell the two apart, which is why
// this asserts bytes.
//
// The limit sits well under the old figure and well over the new one, so it
// fails if the scanner is reinstated and does not fail on incidental runtime
// noise.
func TestCountConnectionsAllocatesLessThanTheScannerItReplaced(t *testing.T) {
	var b strings.Builder
	b.WriteString("header\n")
	const rows = 5000
	for i := 0; i < rows; i++ {
		b.WriteString("row\n")
	}
	path := filepath.Join(t.TempDir(), "net")
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	// One warm-up so the measurement does not carry first-call setup.
	if _, err := countConnections(path); err != nil {
		t.Fatalf("countConnections: %v", err)
	}

	res := testing.Benchmark(func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if _, err := countConnections(path); err != nil {
				t.Fatalf("countConnections: %v", err)
			}
		}
	})
	bytesPerOp := res.MemBytes / uint64(res.N)
	if bytesPerOp > 1024 {
		t.Errorf("countConnections allocates %d B per call for %d entries; "+
			"the line scanner it replaced cost about 4,250 B, so this looks like "+
			"a per-entry buffer has been reintroduced", bytesPerOp, rows)
	}
	t.Logf("countConnections: %d B/op, %d allocs/op", bytesPerOp, res.MemAllocs/uint64(res.N))
}
