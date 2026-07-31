package cliui

import "testing"

func TestChrome_80x24(t *testing.T) {
	c := ComputeChrome(80, 24, 3)
	if c.Stats.H != 3 {
		t.Fatalf("stats H=%d", c.Stats.H)
	}
	if c.Status.H != 3 {
		t.Fatalf("status H=%d", c.Status.H)
	}
	if c.Input.H != 3 {
		t.Fatalf("input H=%d", c.Input.H)
	}
	if c.Sessions.W < 18 || c.Sessions.W > 28 {
		t.Fatalf("sessions W=%d want [18,28]", c.Sessions.W)
	}
	wantViewH := 24 - 3 - 3 - 3 // stats, input, status (gotui min chrome)
	if c.Result.H != wantViewH {
		t.Fatalf("result H=%d want %d", c.Result.H, wantViewH)
	}
	if c.Result.W+c.Sessions.W != 80 {
		t.Fatalf("result+sessions width %d+%d != 80", c.Result.W, c.Sessions.W)
	}
	if c.Result.H < 3 {
		t.Fatalf("result too small: %d", c.Result.H)
	}
	if c.Stats.Y != 0 {
		t.Fatalf("stats should start at top, Y=%d", c.Stats.Y)
	}
	if c.Result.Y != c.Stats.Y+c.Stats.H {
		t.Fatalf("result should sit under stats: result.Y=%d stats bottom=%d", c.Result.Y, c.Stats.Y+c.Stats.H)
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
