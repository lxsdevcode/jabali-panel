package commands

import "testing"

func TestParseReclaimedSpace(t *testing.T) {
	GB := int64(1000 * 1000 * 1000) // docker prints decimal units
	MB := int64(1000 * 1000)
	cases := []struct {
		name string
		out  string
		want int64
	}{
		{
			// The exact shape that reported "Reclaimed 0 B" while freeing 3.5GB.
			name: "gigabytes no space",
			out:  "Deleted Images:\nuntagged: foo\n\nTotal reclaimed space: 3.53GB\n",
			want: int64(3.53 * float64(GB)),
		},
		{
			name: "with space between number and unit",
			out:  "Total reclaimed space: 100 MB\n",
			want: 100 * MB,
		},
		{
			name: "zero",
			out:  "Total reclaimed space: 0B\n",
			want: 0,
		},
		{
			name: "line absent",
			out:  "Deleted Containers:\nabc123\n",
			want: 0,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parseReclaimedSpace(c.out)
			// Allow rounding slack from float size math.
			if d := got - c.want; d > MB || d < -MB {
				t.Errorf("parseReclaimedSpace = %d, want ~%d", got, c.want)
			}
		})
	}
}
