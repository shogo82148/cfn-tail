// Package color is basic color management package for terminal.
package color

import (
	"strconv"
	"strings"
)

// Color is a color on the terminal.
type Color uint16

const (
	// NoColor is no color.
	NoColor Color = 1 << 15
)

const (
	// Foreground colors for ColorScheme.
	_ Color = iota<<bitsForeground | NoColor

	// Black is black foreground color.
	Black

	// Red is red foreground color.
	Red

	// Green is green foreground color.
	Green

	// Yellow is yellow foreground color.
	Yellow

	// Blue is blue foreground color.
	Blue

	// Magenta is magenta foreground color.
	Magenta

	// Cyan is cyan foreground color.
	Cyan

	// White is white foreground color.
	White

	bitsForeground       = 0
	maskForeground       = 0xf
	ansiForegroundOffset = 30 - 1
)

const (
	// Background colors for ColorScheme.
	_ = iota<<bitsBackground | NoColor

	// BackgroundBlack is black background color.
	BackgroundBlack

	// BackgroundRed is red background color.
	BackgroundRed

	// BackgroundGreen is green background color.
	BackgroundGreen

	// BackgroundYellow is yellow background color.
	BackgroundYellow

	// BackgroundBlue is blue background color.
	BackgroundBlue

	// BackgroundMagenta is magenta background color.
	BackgroundMagenta

	// BackgroundCyan is cyan background color.
	BackgroundCyan

	// BackgroundWhite is white background color.
	BackgroundWhite

	bitsBackground       = 4
	maskBackground       = 0xf << bitsBackground
	ansiBackgroundOffset = 40 - 1
)

const (
	// Bold flag makes the font bold.
	Bold     Color = 1<<bitsBold | NoColor
	bitsBold       = 8
	maskBold       = 1 << bitsBold

	// Bright flag makes the foreground color bright.
	Bright                     Color = 1<<bitsBright | NoColor
	bitsBright                       = 9
	maskBright                       = 1 << bitsBright
	ansiBrightForegroundOffset       = 90 - 1

	// BackgroundBright flag makes the background color bright.
	BackgroundBright           Color = 1<<bitsBackgroundBright | NoColor
	bitsBackgroundBright             = 10
	maskBackgroundBright             = 1 << bitsBackgroundBright
	ansiBrightBackgroundOffset       = 100 - 1
)

// Colorize colorizes the text.
func Colorize(text string, color Color) string {
	foreground := color & maskForeground >> bitsForeground
	background := color & maskBackground >> bitsBackground
	bold := color & maskBold
	if foreground == 0 && background == 0 && bold == 0 {
		return text
	}

	var buf strings.Builder
	var tmp [4]byte
	if bold > 0 {
		buf.WriteString("\033[1m")
	}
	if foreground > 0 {
		offset := ansiForegroundOffset
		if (color & maskBright) != 0 {
			offset = ansiBrightForegroundOffset
		}
		buf.WriteString("\033[")
		buf.Write(strconv.AppendInt(tmp[:0], int64(foreground)+int64(offset), 10))
		buf.WriteString("m")
	}
	if background > 0 {
		offset := ansiBackgroundOffset
		if (color & maskBackgroundBright) != 0 {
			offset = ansiBrightBackgroundOffset
		}
		buf.WriteString("\033[")
		buf.Write(strconv.AppendInt(tmp[:0], int64(background)+int64(offset), 10))
		buf.WriteString("m")
	}
	buf.WriteString(text)
	buf.WriteString("\033[0m")
	return buf.String()
}
