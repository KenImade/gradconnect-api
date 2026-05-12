package imagegen

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	_ "image/png"

	"api.gradconnect.com/internal/data"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/draw"
	"golang.org/x/image/font"
)

//go:embed assets/fonts/Fraunces_72pt-Regular.ttf
var regularFontBytes []byte

//go:embed assets/fonts/Fraunces_72pt-Bold.ttf
var boldFontBytes []byte

//go:embed assets/logo.png
var logoBytes []byte

const (
	// Warm neutrals
	neutral50  = "#F6F2E9"
	neutral100 = "#F1ECDF"
	neutral200 = "#E5DECF"
	neutral300 = "#D2C8B4"
	neutral400 = "#9E9079"
	neutral500 = "#766B58"
	neutral600 = "#4E463A"
	neutral700 = "#36302A"
	neutral800 = "#272320"
	neutral900 = "#1C1916"

	// Oxblood / burnt sienna
	primary300 = "#B85A3F"
	primary400 = "#9C4530"
	primary500 = "#8C3826"
	primary600 = "#7E2F1F"
	primary700 = "#682818"
	primary800 = "#4F1F13"
)

type Format string

const (
	FormatTwitter           Format = "twitter"            // 1200×630
	FormatInstagramSquare   Format = "instagram_square"   // 1080×1080
	FormatInstagramPortrait Format = "instagram_portrait" // 1080×1350
	FormatStory             Format = "story"              // 1080×1920 (IG Story / TikTok / Reels)
)

type Generator struct {
	regularFont *truetype.Font
	boldFont    *truetype.Font
	logo        image.Image
}

func New() (*Generator, error) {
	regular, err := truetype.Parse(regularFontBytes)
	if err != nil {
		return nil, fmt.Errorf("parsing regular font: %w", err)
	}
	bold, err := truetype.Parse(boldFontBytes)
	if err != nil {
		return nil, fmt.Errorf("parsing bold font: %w", err)
	}

	rawLogo, _, err := image.Decode(bytes.NewReader(logoBytes))
	if err != nil {
		return nil, fmt.Errorf("decoding logo: %w", err)
	}

	const targetHeight = 60
	bounds := rawLogo.Bounds()
	aspect := float64(bounds.Dx()) / float64(bounds.Dy())
	targetWidth := int(float64(targetHeight) * aspect)

	scaled := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	draw.CatmullRom.Scale(scaled, scaled.Bounds(), rawLogo, bounds, draw.Over, nil)

	return &Generator{regularFont: regular, boldFont: bold, logo: scaled}, nil
}

type OpportunityCard struct {
	Title        string
	EmployerName string
	EmployerLogo image.Image
	Location     string
	Deadline     *data.Date
}

func (g *Generator) face(f *truetype.Font, size float64) font.Face {
	return truetype.NewFace(f, &truetype.Options{Size: size})
}

func (g *Generator) GenerateOpportunityCard(card OpportunityCard, format Format) ([]byte, error) {
	switch format {
	case FormatTwitter:
		return g.renderTwitter(card)
	case FormatInstagramSquare:
		return g.renderInstagramSquare(card)
	case FormatInstagramPortrait:
		return g.renderInstagramPortrait(card)
	case FormatStory:
		return g.renderStory(card)
	default:
		return nil, fmt.Errorf("unknown format %s", format)
	}
}
