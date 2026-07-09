package cpanel

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/repository"
)

// A real, well-formed public key line so ParseAndFingerprint succeeds.
const testAuthKeyLine = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIC+W0tecSNejyTk0MSyH5OJ0skc/eXSRx+9Ht3gpobPB contractor@old"

type fakeSSHRepo struct {
	repository.SSHKeyRepository
	created []*models.SSHKey
}

func (f *fakeSSHRepo) Create(_ context.Context, k *models.SSHKey) error {
	f.created = append(f.created, k)
	return nil
}

// JAB-53: source authorized_keys must NOT grant SSH access by default; only an
// explicit opt-in imports them as active keys.
func TestImportSSHKeys_InertByDefault(t *testing.T) {
	dir := t.TempDir()
	ak := filepath.Join(dir, "authorized_keys")
	if err := os.WriteFile(ak, []byte(testAuthKeyLine+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name        string
		preserve    bool
		wantCreated int
	}{
		{"default: no keys granted", false, 0},
		{"opt-in: keys imported", true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeSSHRepo{}
			parsed := &ParsedTarball{SSHAuthorized: []string{ak}}
			res, err := ImportSSHKeys(context.Background(), repo, parsed, "01USERULID0000000000000000", tc.preserve)
			if err != nil {
				t.Fatalf("ImportSSHKeys: %v", err)
			}
			if len(repo.created) != tc.wantCreated {
				t.Fatalf("created=%d want=%d (skipped=%v)", len(repo.created), tc.wantCreated, res.Skipped)
			}
		})
	}
}

// countingAgent records every command Call receives.
type countingAgent struct {
	calls []string
}

func (c *countingAgent) Call(_ context.Context, command string, _ any) (json.RawMessage, error) {
	c.calls = append(c.calls, command)
	return json.RawMessage(`{}`), nil
}

// JAB-54: source custom SSL cert + private key must NOT be auto-installed by
// default; the reconciler's auto-LE issues a fresh cert instead.
func TestImportSSL_NotInstalledByDefault(t *testing.T) {
	extract := t.TempDir()
	dom := filepath.Join(extract, "apache_tls", "example.com")
	if err := os.MkdirAll(dom, 0o755); err != nil {
		t.Fatal(err)
	}
	// A combined cert+key file so ImportSSL would otherwise install it.
	if err := os.WriteFile(filepath.Join(dom, "combined"),
		[]byte("-----BEGIN CERTIFICATE-----\naGVsbG8=\n-----END CERTIFICATE-----\n-----BEGIN PRIVATE KEY-----\naGVsbG8=\n-----END PRIVATE KEY-----\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name        string
		preserve    bool
		wantInstall bool
	}{
		{"default: source cert not installed", false, false},
		{"opt-in: source cert installed", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ag := &countingAgent{}
			parsed := &ParsedTarball{ExtractDir: extract, SourceUser: "alice"}
			if _, err := ImportSSL(context.Background(), ag, parsed, tc.preserve); err != nil {
				t.Fatalf("ImportSSL: %v", err)
			}
			installed := false
			for _, c := range ag.calls {
				if c == "ssl.install_custom" {
					installed = true
				}
			}
			if installed != tc.wantInstall {
				t.Fatalf("ssl.install_custom called=%v want=%v (calls=%v)", installed, tc.wantInstall, ag.calls)
			}
		})
	}
}
