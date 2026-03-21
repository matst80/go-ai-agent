package terminal

import (
	"bytes"
	"testing"
)

func TestLiveUpdater_Render(t *testing.T) {
	var buf bytes.Buffer
	u := NewLiveUpdaterTo(&buf)

	// Step 1: Initial render
	u.Render("Hello\nWorld")
	expected := "Hello\nWorld\n"
	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}

	// Step 2: Incremental update
	buf.Reset()
	u.Render("Hello\nUniverse")
	// Should move up 1 line and clear from World
	expected = "\033[1A\r\033[JUniverse\n"
	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}

	// Step 3: Same content
	buf.Reset()
	u.Render("Hello\nUniverse")
	expected = ""
	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}

	// Step 4: Multi-line change
	buf.Reset()
	u.Render("Bye\nFolks")
	// Should move up 2 lines and clear
	expected = "\033[2A\r\033[JBye\nFolks\n"
	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}

	// Step 5: Wrapped line
	u.WithWidth(10) // Small width to force wrapping
	buf.Reset()
	u.Render("0123456789ABC") // 13 chars -> 2 rows in width 10
	expected = "\033[2A\r\033[J0123456789ABC\n"
	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}

	// Step 6: Update wrapped line
	buf.Reset()
	u.Render("0123456789XYZ") // Also 13 chars -> 2 rows
	expected = "\033[2A\r\033[J0123456789XYZ\n"
	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
}
