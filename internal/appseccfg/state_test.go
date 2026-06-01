package appseccfg

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadWebmailHosts_MissingFile(t *testing.T) {
	hosts, err := LoadWebmailHosts(filepath.Join(t.TempDir(), "nope.list"))
	if err != nil {
		t.Fatalf("missing file should be (nil, nil); got err=%v", err)
	}
	if hosts != nil {
		t.Fatalf("missing file should be (nil, nil); got hosts=%v", hosts)
	}
}

func TestLoadWebmailHosts_ParsesAndSanitizes(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "hosts.list")
	body := strings.Join([]string{
		"# Comment",
		"",
		"  Mail.Foo.Com  ",
		"autoconfig.foo.com",
		"mail.foo.com", // dup post-lowercase
		"bad host with spaces",
		"weird\"quote",
		"mail.bar.com",
	}, "\n")
	if err := os.WriteFile(tmp, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadWebmailHosts(tmp)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"autoconfig.foo.com", "mail.bar.com", "mail.foo.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got=%v want=%v", got, want)
	}
}

func TestWriteWebmailHosts_AtomicWriteOnDiff(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "out.list")

	changed, err := WriteWebmailHosts(tmp, []string{"mail.a.com", "Mail.A.com", "autoconfig.b.com"})
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("first write must report changed=true")
	}

	body, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "\nautoconfig.b.com\n") {
		t.Fatalf("missing autoconfig.b.com:\n%s", body)
	}
	if !strings.Contains(string(body), "\nmail.a.com\n") {
		t.Fatalf("missing mail.a.com (lowercase dedupe):\n%s", body)
	}

	// Re-write with permuted input but same effective set → unchanged.
	changed2, err := WriteWebmailHosts(tmp, []string{"autoconfig.b.com", "mail.a.com"})
	if err != nil {
		t.Fatal(err)
	}
	if changed2 {
		t.Fatal("idempotent re-write must report changed=false")
	}

	// New host → changed.
	changed3, err := WriteWebmailHosts(tmp, []string{"mail.a.com", "autoconfig.b.com", "mail.c.com"})
	if err != nil {
		t.Fatal(err)
	}
	if !changed3 {
		t.Fatal("adding a host must report changed=true")
	}
}
