package imagegen

import (
	"bytes"
	"fmt"

	"github.com/fogleman/gg"
)

// Instagram portrait feed post. 4:5.
// Same vertical structure as square but with more breathing room.
func (g *Generator) renderInstagramPortrait(card OpportunityCard) ([]byte, error) {
	const (
		W       = 1080
		H       = 1350
		padding = 80
		siteURL = "gradconnect.ng"

		headerY       = 100.0
		eyebrowY      = 200.0
		companyLogoY  = 290.0
		companyLogoSz = 160.0
		titleY        = 540.0
		titleMaxRunes = 65
		titleSize     = 68.0
	)

	dc := gg.NewContext(W, H)

	g.drawBackground(dc, W, H)

	// Header: logo left, URL right
	logoBounds := g.logo.Bounds()
	logoH := float64(logoBounds.Dy())
	dc.DrawImage(g.logo, padding, int(headerY-logoH/2))

	dc.SetFontFace(g.face(g.regularFont, 26))
	dc.SetHexColor(neutral600)
	dc.DrawStringAnchored(siteURL, W-padding, headerY, 1.0, 0.5)

	// Eyebrow (centered)
	dc.SetFontFace(g.face(g.boldFont, 24))
	dc.SetHexColor(primary600)
	dc.DrawStringAnchored("GRADUATE OPPORTUNITY", W/2, eyebrowY, 0.5, 0.5)

	// Company logo (centered)
	g.drawEmployerLogoBox(dc,
		(W-companyLogoSz)/2, companyLogoY,
		companyLogoSz, card.EmployerLogo)

	// Title (centered, wrapped)
	title := truncateTitle(card.Title, titleMaxRunes)
	dc.SetFontFace(g.face(g.boldFont, titleSize))
	dc.SetHexColor(neutral900)

	titleWrapWidth := float64(W - 2*padding)
	dc.DrawStringWrapped(
		title,
		W/2, titleY,
		0.5, 0,
		titleWrapWidth,
		1.25,
		gg.AlignCenter,
	)

	// Employer (below title)
	wrappedTitleLines := dc.WordWrap(title, titleWrapWidth)
	titleHeight := float64(len(wrappedTitleLines)) * titleSize * 1.25
	employerY := titleY + titleHeight + 80

	dc.SetFontFace(g.face(g.regularFont, 40))
	dc.SetHexColor(neutral700)
	dc.DrawStringAnchored(card.EmployerName, W/2, employerY, 0.5, 0.5)

	// Meta row
	g.drawMetaRow(dc, W, H, padding, card.Location, card.Deadline)

	var buf bytes.Buffer
	if err := dc.EncodePNG(&buf); err != nil {
		return nil, fmt.Errorf("encoding PNG: %w", err)
	}
	return buf.Bytes(), nil
}
