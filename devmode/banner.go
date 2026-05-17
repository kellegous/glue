package devmode

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"strings"

	"github.com/kellegous/tdfiglet"
)

//go:embed fonts
var fonts embed.FS

type BannerOptions struct {
	chooser func(names []string) string
}

type BannerOption func(*BannerOptions)

func WithFont(font string) BannerOption {
	return func(o *BannerOptions) {
		o.chooser = func(_ []string) string {
			return font
		}
	}
}

func defaultOptions() BannerOptions {
	return BannerOptions{
		chooser: chooseRandom,
	}
}

func chooseRandom(names []string) string {
	return names[rand.Intn(len(names))]
}

func BannerFonts() ([]string, error) {
	files, err := fs.ReadDir(fonts, "fonts")
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(files))
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".tdf") {
			continue
		}
		names = append(names, file.Name())
	}
	return names, nil
}

func renderBanner(w io.Writer, fontName string, text string) error {
	sub, err := fs.Sub(fonts, "fonts")
	if err != nil {
		return err
	}

	r, err := sub.Open(fontName)
	if err != nil {
		return err
	}
	defer r.Close()

	font, err := tdfiglet.LoadFont(r)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "\n%s\n", font.Render(text)); err != nil {
		return err
	}

	return nil
}
