package imagegen

import (
	"image"
	"strings"
	"unicode/utf8"

	"api.gradconnect.com/internal/data"
	"github.com/fogleman/gg"
	"golang.org/x/image/draw"
)

func (g *Generator) drawBackground(dc *gg.Context, w, h int) {
	dc.SetHexColor(neutral100)
	dc.Clear()
	dc.SetHexColor(neutral200)
	dc.DrawRectangle(0, 0, float64(w), float64(h))
	dc.Fill()
}

// drawEmployerLogoBox draws the employer's logo fitted within a square box,
// preserving aspect ratio and centering the result. If no logo is provided,
// draws a neutral placeholder. The (x, y) is the top-left of the box.
func (g *Generator) drawEmployerLogoBox(dc *gg.Context, x, y, size float64, logo image.Image) {
	if logo == nil {
		dc.SetHexColor(neutral200)
		dc.DrawRectangle(x, y, size, size)
		dc.Fill()
		dc.SetHexColor(neutral300)
		dc.SetLineWidth(1)
		dc.DrawRectangle(x, y, size, size)
		dc.Stroke()
		return
	}

	fitted := fitInBox(logo, int(size))
	fb := fitted.Bounds()

	// Center the fitted image within the box
	dx := x + (size-float64(fb.Dx()))/2
	dy := y + (size-float64(fb.Dy()))/2

	dc.DrawImage(fitted, int(dx), int(dy))
}

// fitInBox returns the source image rescaled to fit within a box of the given
// size, preserving aspect ratio. The longer dimension of the result equals
// boxSize; the shorter is scaled proportionally.
func fitInBox(src image.Image, boxSize int) image.Image {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	if sw == 0 || sh == 0 {
		return src
	}

	var w, h int
	if sw >= sh {
		w = boxSize
		h = boxSize * sh / sw
	} else {
		h = boxSize
		w = boxSize * sw / sh
	}

	// Skip resize if the source already fits exactly
	if w == sw && h == sh {
		return src
	}

	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	return dst
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
// if truncation occurred. Operates on runes to handle multibyte UTF-8 safely.
func truncateTitle(s string, maxRunes int) string {
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	runes := []rune(s)
	return strings.TrimSpace(string(runes[:maxRunes-1])) + "…"
}
