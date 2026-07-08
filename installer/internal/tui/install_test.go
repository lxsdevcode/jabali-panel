package tui

import "testing"

func TestStripANSI(t *testing.T) {
	in := "\x1b[1;34m[i]\x1b[0m installing PowerDNS"
	if got := stripANSI(in); got != "[i] installing PowerDNS" {
		t.Errorf("stripANSI = %q", got)
	}
}

func TestPhaseFromLine(t *testing.T) {
	p, ok := phaseFromLine("\x1b[1;34m[i]\x1b[0m installing PowerDNS")
	if !ok || p != "installing PowerDNS" {
		t.Errorf("phaseFromLine = %q ok=%v", p, ok)
	}
	if _, ok := phaseFromLine("\x1b[1;32m[✓]\x1b[0m done"); ok {
		t.Error("a [✓] ok-line should not set a new phase")
	}
	if _, ok := phaseFromLine("plain line"); ok {
		t.Error("a non-marker line should not set a phase")
	}
}
