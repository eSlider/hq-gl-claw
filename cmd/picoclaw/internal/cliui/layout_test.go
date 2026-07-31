package cliui

import "testing"

func TestChrome_80x24(t *testing.T) {
	c := ComputeChrome(80, 24, 3)
	if c.Input.H != 3 {
		t.Fatalf("input H=%d", c.Input.H)
	}
	if c.Sessions.W < 18 || c.Sessions.W > 28 {
		t.Fatalf("sessions W=%d want [18,28]", c.Sessions.W)
	}
	wantViewH := 24 - 3 // input only (status lives in input title)
	if c.Result.H != wantViewH {
		t.Fatalf("result H=%d want %d", c.Result.H, wantViewH)
	}
	if c.Result.W+c.Sessions.W != 80 {
		t.Fatalf("result+sessions width %d+%d != 80", c.Result.W, c.Sessions.W)
	}
	if c.Result.Y != 0 {
		t.Fatalf("result should start at top, Y=%d", c.Result.Y)
	}
	if c.Sessions.H != c.Result.H {
		t.Fatalf("sessions H=%d should match result H=%d", c.Sessions.H, c.Result.H)
	}
	if c.Input.Y != c.Result.H || c.Input.W != 80 {
		t.Fatalf("input should be full-width under result: %+v", c.Input)
	}
}

func TestChrome_MinHeight(t *testing.T) {
	c := ComputeChrome(40, 6, 3)
	if c.Result.H < 3 {
		t.Fatalf("viewport must clamp to >=3, got %d", c.Result.H)
	}
}

func TestChrome_Resize(t *testing.T) {
	narrow := ComputeChrome(60, 24, 3)
	wide := ComputeChrome(120, 24, 3)
	if wide.Result.W <= narrow.Result.W {
		t.Fatalf("widening should grow result: narrow=%d wide=%d", narrow.Result.W, wide.Result.W)
	}
	if wide.Sessions.W > 28 {
		t.Fatalf("sessions past max: %d", wide.Sessions.W)
	}
}

func TestHitTestPane_NoTopStats(t *testing.T) {
	c := ComputeChrome(80, 24, 3)
	if HitTestPane(c, c.Result.X+1, 0) != FocusResult {
		t.Fatal("top-left should be result")
	}
	// Bottom rows are the input pane (status is in its title, not a separate widget).
	if HitTestPane(c, 1, 23) != FocusInput {
		t.Fatal("bottom should be input")
	}
}
