package imagegen

import (
	"bytes"
	"fmt"

	"github.com/fogleman/gg"
)

// Twitter / X large card, also used for LinkedIn and Facebook OG.
// Landscape 1.91:1.
func (g *Generator) renderTwitter(card OpportunityCard) ([]byte, error) {
	const (
		W       = 1200
		H       = 630
		padding = 80
		siteURL = "gradconnect.ng"

		headerY       = 95.0
		contentY      = 230.0
		companyLogoSz = 120.0
		titleGap      = 32.0
		titleMaxRunes = 55
		titleSize     = 56.0
	)

	titleX := float64(padding) + companyLogoSz + titleGap

	dc := gg.NewContext(W, H)

	// Background + accent bar
	g.drawBackground(dc, W, H)

	// === HEADER ROW: logo | eyebrow | URL ===

	logoBounds := g.logo.Bounds()
	logoH := float64(logoBounds.Dy())
	logoW := float64(logoBounds.Dx())
	dc.DrawImage(g.logo, padding, int(headerY-logoH/2))

	dc.SetFontFace(g.face(g.boldFont, 22))
	dc.SetHexColor(primary600)
	dc.DrawStringAnchored("GRADUATE OPPORTUNITY",
		float64(padding)+logoW+32, headerY, 0.0, 0.5)

	dc.SetFontFace(g.face(g.regularFont, 24))
	dc.SetHexColor(neutral600)
	dc.DrawStringAnchored(siteURL, W-padding, headerY, 1.0, 0.5)

	// === CONTENT ROW: company logo + title ===

	g.drawEmployerLogoBox(dc, padding, contentY, companyLogoSz, card.EmployerLogo)

	// Title — truncated, fixed size, wrapped
	title := truncateTitle(card.Title, titleMaxRunes)
	dc.SetFontFace(g.face(g.boldFont, titleSize))
	dc.SetHexColor(neutral900)

	titleWrapWidth := W - titleX - padding
	wrappedLines := dc.WordWrap(title, titleWrapWidth)
	const titleLineSpacing = 1.2
	titleHeight := float64(len(wrappedLines)) * titleSize * titleLineSpacing

	dc.DrawStringWrapped(
		title,
		titleX, contentY,
		0, 0,
		titleWrapWidth,
		titleLineSpacing,
		gg.AlignLeft,
	)

	// Employer — below the taller of title or logo
	titleBottom := contentY + titleHeight
	logoBottom := contentY + companyLogoSz
	employerY := titleBottom
	if logoBottom > titleBottom {
		employerY = logoBottom
	}
	employerY += 50

	dc.SetFontFace(g.face(g.regularFont, 32))
	dc.SetHexColor(neutral700)
	dc.DrawString(card.EmployerName, titleX, employerY)

	// === META ROW: location + deadline ===

	g.drawMetaRow(dc, W, H, padding, card.Location, card.Deadline)

	var buf bytes.Buffer
	if err := dc.EncodePNG(&buf); err != nil {
		return nil, fmt.Errorf("encoding PNG: %w", err)
	}
	return buf.Bytes(), nil
}
