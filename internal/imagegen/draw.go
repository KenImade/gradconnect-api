package imagegen

import (
	"image"
	"strings"
	"unicode/utf8"

	"api.gradconnect.com/internal/data"
	"github.com/fogleman/gg"
)

func (g *Generator) drawBackground(dc *gg.Context, w, h int) {
	dc.SetHexColor(neutral100)
	dc.Clear()
	dc.SetHexColor(neutral200)
	dc.DrawRectangle(0, 0, float64(w), float64(h))
	dc.Fill()
}

func (g *Generator) drawEmployerLogoBox(dc *gg.Context, x, y, size float64, logo image.Image) {
	if logo != nil {
		// fit-in-box logic
		dc.DrawImage(logo, int(x), int(y))
		return
	}
	dc.SetHexColor(neutral200)
	dc.DrawRectangle(x, y, size, size)
	dc.Fill()
	dc.SetHexColor(neutral300)
	dc.SetLineWidth(1)
	dc.DrawRectangle(x, y, size, size)
	dc.Stroke()
}

func (g *Generator) drawMetaRow(dc *gg.Context, w, h, padding float64, location string, deadline *data.Date) {
	dc.SetHexColor(neutral300)
	dc.DrawRectangle(padding, h-padding-50, w-2*padding, 1)
	dc.Fill()

	dc.SetFontFace(g.face(g.regularFont, 26))
	dc.SetHexColor(neutral500)

	if location != "" {
		dc.DrawString("Location: "+location, padding, h-padding)
	}

	if deadline != nil {
		deadlineText := "Apply by " + deadline.Format("02 Jan 2006")
		dc.DrawStringAnchored(deadlineText, w-padding, h-padding, 1.0, 0.0)
	}
}

// truncateTitle limits the title to maxRunes characters, appending an ellipsis
// if truncation occurred. Operates on runes to handle multi-byte UTF-8 safely.
func truncateTitle(s string, maxRunes int) string {
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	runes := []rune(s)
	return strings.TrimSpace(string(runes[:maxRunes-1])) + "…"
}
