package imagegen

import (
	"bytes"
	"fmt"

	"github.com/fogleman/gg"
)

// Vertical Story format for Instagram Stories, TikTok, and Reels. 9:16.
//
// Safe zone considerations:
//   - Top ~250px is covered by profile pic / username
//   - Bottom ~340px is covered by caption + action buttons
//   - Critical content lands between y=320 and y=1580
//
// We intentionally place the brand mark in the "unsafe" top strip — it's
// fine if it's partially obscured, since the title is the actual payload.
func (g *Generator) renderStory(card OpportunityCard) ([]byte, error) {
	const (
		W       = 1080
		H       = 1920
		padding = 80
		siteURL = "gradconnect.ng"

		// Safe-zone landmarks
		safeTop    = 320.0
		safeBottom = 1580.0

		eyebrowY      = 380.0
		companyLogoY  = 500.0
		companyLogoSz = 200.0
		titleY        = 820.0
		titleMaxRunes = 70
		titleSize     = 80.0
	)

	dc := gg.NewContext(W, H)

	// Background + accent bar
	g.drawBackground(dc, W, H)

	// === Brand mark (top — partially obscured by IG/TikTok UI, that's OK) ===

	logoBounds := g.logo.Bounds()
	logoH := float64(logoBounds.Dy())
	dc.DrawImage(g.logo, padding, int(150-logoH/2))

	// === SAFE-ZONE CONTENT STARTS HERE ===

	// Eyebrow label
	dc.SetFontFace(g.face(g.boldFont, 28))
	dc.SetHexColor(primary600)
	dc.DrawStringAnchored("GRADUATE OPPORTUNITY", W/2, eyebrowY, 0.5, 0.5)

	// Company logo (centered, large)
	g.drawEmployerLogoBox(dc,
		(W-companyLogoSz)/2, companyLogoY,
		companyLogoSz, card.EmployerLogo)

	// Title (centered, wrapped, large)
	title := truncateTitle(card.Title, titleMaxRunes)
	dc.SetFontFace(g.face(g.boldFont, titleSize))
	dc.SetHexColor(neutral900)

	titleWrapWidth := float64(W - 2*padding)
	dc.DrawStringWrapped(
		title,
		W/2, titleY,
		0.5, 0,
		titleWrapWidth,
		1.2,
		gg.AlignCenter,
	)

	// Employer (below title)
	wrappedTitleLines := dc.WordWrap(title, titleWrapWidth)
	titleHeight := float64(len(wrappedTitleLines)) * titleSize * 1.2
	employerY := titleY + titleHeight + 100

	dc.SetFontFace(g.face(g.regularFont, 44))
	dc.SetHexColor(neutral700)
	dc.DrawStringAnchored(card.EmployerName, W/2, employerY, 0.5, 0.5)

	// === BOTTOM OF SAFE ZONE: meta info ===
	// Instead of using drawMetaRow (which anchors to canvas bottom),
	// render meta inside the safe zone manually.

	metaY := safeBottom - 80

	// Hairline divider
	dc.SetHexColor(neutral300)
	dc.DrawRectangle(padding, metaY-50, W-2*padding, 1)
	dc.Fill()

	// Location + deadline, centered on metaY
	dc.SetFontFace(g.face(g.regularFont, 32))
	dc.SetHexColor(neutral500)

	if card.Location != "" {
		dc.DrawString(card.Location, padding, metaY)
	}
	if card.Deadline != nil {
		deadlineText := "Apply by " + card.Deadline.Format("02 Jan 2006")
		dc.DrawStringAnchored(deadlineText, W-padding, metaY, 1.0, 0.0)
	}

	// === URL at very bottom of safe zone (under-the-fold but visible briefly) ===

	dc.SetFontFace(g.face(g.regularFont, 28))
	dc.SetHexColor(neutral600)
	dc.DrawStringAnchored(siteURL, W/2, safeBottom-20, 0.5, 0.5)

	var buf bytes.Buffer
	if err := dc.EncodePNG(&buf); err != nil {
		return nil, fmt.Errorf("encoding PNG: %w", err)
	}
	return buf.Bytes(), nil
}
