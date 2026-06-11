package certbot

import "testing"

func TestRawErrorTail(t *testing.T) {
	out := "Saving debug log to /var/log/letsencrypt/letsencrypt.log\n" +
		"- - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -\n" +
		"\n" +
		"Certbot failed to authenticate some domains (authenticator: webroot).\n" +
		"Error: The requested url returned 404\n"
	got := RawErrorTail(out, 6)
	if got == "" {
		t.Fatal("expected non-empty tail")
	}
	// boilerplate dashes dropped; real lines kept
	if want := "404"; !contains(got, want) {
		t.Errorf("tail missing %q: %q", want, got)
	}
	if contains(got, "- - -") {
		t.Errorf("boilerplate framing not dropped: %q", got)
	}
}

func TestRawErrorTail_Empty(t *testing.T) {
	if RawErrorTail("\n\n   \n", 6) != "" {
		t.Error("blank-only input should yield empty tail")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
