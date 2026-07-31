package cliui

import "testing"

func TestFocus_TabCycle(t *testing.T) {
	f := FocusInput
	f = f.Next()
	if f != FocusResult {
		t.Fatalf("got %v", f)
	}
	f = f.Next()
	if f != FocusSessions {
		t.Fatalf("got %v", f)
	}
	f = f.Next()
	if f != FocusInput {
		t.Fatalf("got %v", f)
	}
}

func TestFocus_ShiftTabReverse(t *testing.T) {
	f := FocusInput
	f = f.Prev()
	if f != FocusSessions {
		t.Fatalf("got %v", f)
	}
	f = f.Prev()
	if f != FocusResult {
		t.Fatalf("got %v", f)
	}
	f = f.Prev()
	if f != FocusInput {
		t.Fatalf("got %v", f)
	}
}

func TestFocus_StreamingAllowsTab(t *testing.T) {
	if !AllowKey(FocusInput, true, "tab") {
		t.Fatal("tab should be allowed while streaming")
	}
	if AllowKey(FocusInput, true, "enter") {
		t.Fatal("enter/submit blocked while streaming")
	}
	if !AllowKey(FocusResult, true, "j") {
		t.Fatal("scroll allowed while streaming")
	}
}
