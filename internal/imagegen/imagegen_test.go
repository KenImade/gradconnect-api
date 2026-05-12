package imagegen

import (
	"fmt"
	"os"
	"testing"
	"time"

	"api.gradconnect.com/internal/data"
)

func TestGenerateOpportunityCard_Visual(t *testing.T) {
	g, err := New()
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	deadline := data.Date{Time: time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)}

	cases := []struct {
		name string
		card OpportunityCard
	}{
		{
			name: "typical",
			card: OpportunityCard{
				Title:        "Software Engineering Graduate Trainee",
				EmployerName: "Coronation Merchant Bank",
				Location:     "Lagos, Nigeria",
				Deadline:     &deadline,
			},
		},
		{
			name: "long_title",
			card: OpportunityCard{
				Title:        "Senior Backend Software Engineering Graduate Trainee Programme for 2026",
				EmployerName: "Trium Limited",
				Location:     "Lagos, Nigeria",
				Deadline:     &deadline,
			},
		},
	}

	formats := []Format{
		FormatTwitter,
		FormatInstagramSquare,
		FormatInstagramPortrait,
		FormatStory,
	}

	if err := os.MkdirAll("testdata", 0755); err != nil {
		t.Fatal(err)
	}

	for _, tc := range cases {
		for _, f := range formats {
			t.Run(string(f)+"_"+tc.name, func(t *testing.T) {
				img, err := g.GenerateOpportunityCard(tc.card, f)
				if err != nil {
					t.Fatalf("generate: %v", err)
				}
				path := fmt.Sprintf("testdata/card_%s_%s.png", f, tc.name)
				if err := os.WriteFile(path, img, 0644); err != nil {
					t.Fatal(err)
				}
				t.Logf("wrote %s (%d bytes)", path, len(img))
			})
		}
	}
}
