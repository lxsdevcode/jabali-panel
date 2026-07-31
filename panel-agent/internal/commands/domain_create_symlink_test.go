package commands

import (
	"bytes"
	"strings"
	"testing"
	"text/template"
)

// JAB-183. Docroots are 2750 <user>:www-data and homes 0750 <user>:www-data, so
// the nginx worker can read every tenant's tree. The only thing keeping one
// tenant's vhost from serving another's files is that `root` points into their
// own tree -- and a symlink defeats that unless nginx is told not to follow it.
//
// This asserts the directive survives future template edits. If it is removed,
// a tenant can symlink to a neighbour's wp-config.php, name it .txt so the URI
// never reaches the PHP location, and read it over HTTP.
func TestVhostTemplate_DisablesCrossOwnerSymlinks(t *testing.T) {
	t.Parallel()
	vd := vhostData{
		Domain: "example.com", DocRoot: "/home/u/domains/example.com/public_html",
		HasPHP: true, PHPVersion: "8.3", Username: "u", IsEnabled: true,
		FPMSocket: "/run/php/jabali-u/fpm.sock",
	}
	tmpl := template.Must(template.New("vhost").Parse(vhostTemplate))
	var b bytes.Buffer
	if err := tmpl.Execute(&b, vd); err != nil {
		t.Fatal(err)
	}
	got := b.String()

	if !strings.Contains(got, "disable_symlinks if_not_owner from=$document_root;") {
		t.Error("vhost is missing `disable_symlinks if_not_owner from=$document_root` -- " +
			"a tenant can symlink into another tenant's docroot and have nginx serve it")
	}
	// if_not_owner, not a blanket `on`: same-owner symlinks inside a tenant's own
	// tree (Laravel public/storage, release-dir deploys) must keep working.
	if strings.Contains(got, "disable_symlinks on") {
		t.Error("blanket `disable_symlinks on` breaks same-owner symlinks tenants legitimately use")
	}
}

// A disabled domain serves the parked page from /var/www/jabali-disabled and
// never exposes the tenant docroot, so the directive is not required there.
func TestVhostTemplate_DisabledDomainDoesNotServeDocRoot(t *testing.T) {
	t.Parallel()
	vd := vhostData{
		Domain: "example.com", DocRoot: "/home/u/domains/example.com/public_html",
		Username: "u", IsEnabled: false,
	}
	tmpl := template.Must(template.New("vhost").Parse(vhostTemplate))
	var b bytes.Buffer
	if err := tmpl.Execute(&b, vd); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(b.String(), "root "+vd.DocRoot+";") {
		t.Error("disabled domain must not root into the tenant docroot")
	}
}
