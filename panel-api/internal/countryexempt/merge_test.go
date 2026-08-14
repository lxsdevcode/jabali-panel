package countryexempt

import (
	"testing"
)

func TestMergeCIDRs_SiblingCoalesce(t *testing.T) {
	got := mergeCIDRs([]string{"1.0.0.0/25", "1.0.0.128/25"})
	if len(got) != 1 || got[0] != "1.0.0.0/24" {
		t.Fatalf("got %v, want [1.0.0.0/24]", got)
	}
}

func TestMergeCIDRs_DropsContained(t *testing.T) {
	got := mergeCIDRs([]string{"1.0.0.0/8", "1.2.3.0/24", "1.2.3.4/32"})
	if len(got) != 1 || got[0] != "1.0.0.0/8" {
		t.Fatalf("got %v, want [1.0.0.0/8]", got)
	}
}

func TestMergeCIDRs_MultiLevelCascade(t *testing.T) {
	// Four /26s forming a /24 must collapse all the way up.
	got := mergeCIDRs([]string{
		"10.0.0.0/26", "10.0.0.64/26", "10.0.0.128/26", "10.0.0.192/26",
	})
	if len(got) != 1 || got[0] != "10.0.0.0/24" {
		t.Fatalf("got %v, want [10.0.0.0/24]", got)
	}
}

func TestMergeCIDRs_FamiliesStaySeparate(t *testing.T) {
	got := mergeCIDRs([]string{"2001:db8::/64", "2001:db8:0:1::/64", "1.0.0.0/24"})
	if len(got) != 2 {
		t.Fatalf("v4 and v6 must not merge: %v", got)
	}
	if got[0] != "1.0.0.0/24" || got[1] != "2001:db8::/63" {
		t.Fatalf("got %v", got)
	}
}

func TestMergeCIDRs_SkipsGarbageAndNormalizes(t *testing.T) {
	got := mergeCIDRs([]string{"not-a-cidr", "1.2.3.4/24", ""})
	if len(got) != 1 || got[0] != "1.2.3.0/24" {
		t.Fatalf("got %v, want [1.2.3.0/24] (masked)", got)
	}
}

func TestNormalizeCIDR(t *testing.T) {
	cases := map[string]string{
		"1.2.3.4":       "1.2.3.4/32",
		"1.2.3.4/24":    "1.2.3.0/24",
		"2001:db8::1":   "2001:db8::1/128",
		"2001:db8::1/64": "2001:db8::/64",
	}
	for in, want := range cases {
		got, err := NormalizeCIDR(in)
		if err != nil || got != want {
			t.Fatalf("NormalizeCIDR(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "nonsense", "1.2.3.256", "10.0.0.0/33"} {
		if _, err := NormalizeCIDR(bad); err == nil {
			t.Fatalf("NormalizeCIDR(%q) must fail", bad)
		}
	}
}