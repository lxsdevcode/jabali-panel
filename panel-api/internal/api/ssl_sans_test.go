package api

import (
	"reflect"
	"testing"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/models"
)

func TestWebCertSANsForDomain(t *testing.T) {
	cases := []struct {
		name string
		dom  *models.Domain
		want []string
	}{
		{
			name: "plain (no email)",
			dom:  &models.Domain{Name: "example.com", EmailEnabled: false},
			want: []string{"example.com", "www.example.com"},
		},
		{
			name: "email adds mail/autoconfig/autodiscover",
			dom:  &models.Domain{Name: "example.com", EmailEnabled: true},
			want: []string{"example.com", "www.example.com", "mail.example.com", "autoconfig.example.com", "autodiscover.example.com"},
		},
		{
			name: "mta-sts adds its SAN",
			dom:  &models.Domain{Name: "example.com", EmailEnabled: true, MTASTSEnabled: true},
			want: []string{"example.com", "www.example.com", "mail.example.com", "autoconfig.example.com", "autodiscover.example.com", "mta-sts.example.com"},
		},
		{
			name: "SkipAutoSAN keeps only base pair",
			dom:  &models.Domain{Name: "example.com", EmailEnabled: true, MTASTSEnabled: true, SkipAutoSAN: true},
			want: []string{"example.com", "www.example.com"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := webCertSANsForDomain(tc.dom); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestMailCertSANs(t *testing.T) {
	want := []string{"mail.example.com", "autoconfig.example.com", "autodiscover.example.com", "mta-sts.example.com"}
	if got := mailCertSANs("Example.COM"); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
}
