package shellrun

import (
	"strings"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"

	fileenc "reasonix/internal/base/fileutil/encoding"
)

const codePageLine = "FIND: 参数格式不正确\r\n"

func gbkBytes(t *testing.T, s string) []byte {
	t.Helper()
	b, err := simplifiedchinese.GB18030.NewEncoder().Bytes([]byte(s))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// A Windows console tool answers in the machine's code page. The bytes used to
// be kept as a Go string unchanged, and JSON then coerced every invalid one to
// U+FFFD: a run that failed four times told the model nothing about why.
func TestShellOutputInTheMachineCodePageIsReadable(t *testing.T) {
	if got := decodeShellOutput(gbkBytes(t, codePageLine), fileenc.Cut{}); got != codePageLine {
		t.Fatalf("decodeShellOutput = %q, want %q", got, codePageLine)
	}
}

// UTF-8 output is passed through, less a character the buffer cut in half —
// re-reading that as the code page would invent mojibake where truncation was
// the only problem.
func TestUTF8OutputIsNotReinterpreted(t *testing.T) {
	full := "参数格式不正确 ok\n"
	if got := decodeShellOutput([]byte(full), fileenc.Cut{}); got != full {
		t.Fatalf("valid UTF-8 was rewritten: %q", got)
	}
	if got := decodeShellOutput([]byte(full)[1:], fileenc.Cut{Head: true}); got != full[3:] {
		t.Fatalf("a front-truncated tail was reinterpreted: %q", got)
	}
	if tail := []byte(full)[:len(full)-4]; decodeShellOutput(tail, fileenc.Cut{Tail: true}) != string(tail) {
		t.Fatal("a back-truncated tail was reinterpreted")
	}
	if got := decodeShellOutput(nil, fileenc.Cut{}); got != "" {
		t.Fatalf("empty output = %q", got)
	}
	if got := decodeShellOutput([]byte("plain ascii"), fileenc.Cut{}); !strings.Contains(got, "ascii") {
		t.Fatalf("ascii = %q", got)
	}
}

// The collector is what a run's output actually passes through, so the decode
// has to sit there rather than in a helper the caller could stop using.
func TestTheCollectorDecodesWhatTheChildWrote(t *testing.T) {
	gbk := gbkBytes(t, codePageLine)
	c := newOutputCollector(1<<20, 1<<10)
	if _, err := c.combined.Write(gbk); err != nil {
		t.Fatal(err)
	}
	if _, err := c.tail.Write(gbk); err != nil {
		t.Fatal(err)
	}
	if got := c.combinedString(); got != codePageLine {
		t.Fatalf("combined = %q, want %q", got, codePageLine)
	}
	if got := c.tailString(); got != codePageLine {
		t.Fatalf("tail = %q, want %q", got, codePageLine)
	}
}

// Output cut short inside a code-page character is still code-page text. The
// cut fails the byte-for-byte round trip a whole file must pass before it may
// be rewritten, and output is only ever read.
func TestCodePageOutputCutMidCharacterIsReadable(t *testing.T) {
	gbk := gbkBytes(t, codePageLine)
	cut := gbk[:len(gbk)-3]
	if got := decodeShellOutput(cut, fileenc.Cut{Tail: true}); !strings.HasPrefix(got, "FIND: 参数格式不正") {
		t.Fatalf("decodeShellOutput = %q, want the code-page text up to the cut", got)
	}
}
