package cmd

import "testing"

func TestReportLabels(t *testing.T) {
	tests := []struct {
		name   string
		labels []string
		want   string
	}{
		{name: "member suffix", labels: []string{"async-daily/member"}, want: "async-daily/report"},
		{name: "plain prefix", labels: []string{"daily-update"}, want: "daily-update/report"},
		{name: "fallback default", labels: nil, want: "daily-update/report"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reportLabels(tt.labels)
			if len(got) != 1 || got[0] != tt.want {
				t.Fatalf("reportLabels(%v) = %v, want [%s]", tt.labels, got, tt.want)
			}
		})
	}
}
