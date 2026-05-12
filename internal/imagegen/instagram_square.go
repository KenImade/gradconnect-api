package imagegen

import (
	"bytes"
	"fmt"

	"github.com/fogleman/gg"
)

// Instagram square feed post. 1:1.
// Vertical stack: header → company logo → title → employer → meta.
func (g *Generator) renderInstagramSquare(card OpportunityCard) ([]byte, error) {
	const (
		W       = 1080
		H       = 1080
		padding = 72
		siteURL = "gradconnect.ng"

		headerY       = 90.0
		companyLogoY  = 200.0
		companyLogoSz = 140.0
		titleY        = 400.0
		titleMaxRunes = 60
		titleSize     = 64.0
	)

	dc := gg.NewContext(W, H)

	// Background + accent bar
	g.drawBackground(dc, W, H)

	// === HEADER: logo left, URL right ===

	logoBounds := g.logo.Bounds()
	logoH := float64(logoBounds.Dy())
	dc.DrawImage(g.logo, padding, int(headerY-logoH/2))

	dc.SetFontFace(g.face(g.regularFont, 24))
	dc.SetHexColor(neutral600)
	dc.DrawStringAnchored(siteURL, W-padding, headerY, 1.0, 0.5)

	// === EYEBROW LABEL (centered, below header) ===

	dc.SetFontFace(g.face(g.boldFont, 22))
	dc.SetHexColor(primary600)
	dc.DrawStringAnchored("GRADUATE OPPORTUNITY", W/2, 160, 0.5, 0.5)

	// === COMPANY LOGO (centered) ===

	g.drawEmployerLogoBox(dc,
		(W-companyLogoSz)/2, companyLogoY,
		companyLogoSz, card.EmployerLogo)

	// === TITLE (centered, wrapped) ===

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

	// === EMPLOYER NAME (centered) ===

	wrappedTitleLines := dc.WordWrap(title, titleWrapWidth)
	titleHeight := float64(len(wrappedTitleLines)) * titleSize * 1.25
	employerY := titleY + titleHeight + 60

	dc.SetFontFace(g.face(g.regularFont, 36))
	dc.SetHexColor(neutral700)
	dc.DrawStringAnchored(card.EmployerName, W/2, employerY, 0.5, 0.5)

	// === META ROW ===

	g.drawMetaRow(dc, W, H, padding, card.Location, card.Deadline)

	var buf bytes.Buffer
	if err := dc.EncodePNG(&buf); err != nil {
		return nil, fmt.Errorf("encoding PNG: %w", err)
	}
	return buf.Bytes(), nil
}
