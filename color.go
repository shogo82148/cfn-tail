package main

import (
	"strconv"
	"strings"
)

// very basic color library
type color uint16

const (
	// No color
	noColor color = 1 << 15
)

const (
	// Foreground colors for ColorScheme.
	_ color = iota<<bitsForeground | noColor
	black
	red
	green
	yellow
	blue
	magenta
	cyan
	white

	bitsForeground       = 0
	maskForeground       = 0xf
	ansiForegroundOffset = 30 - 1
)

const (
	// Background colors for ColorScheme.
	_ = iota<<bitsBackground | noColor
	backgroundBlack
	backgroundRed
	backgroundGreen
	backgroundYellow
	backgroundBlue
	backgroundMagenta
	backgroundCyan
	backgroundWhite

	bitsBackground       = 4
	maskBackground       = 0xf << bitsBackground
	ansiBackgroundOffset = 40 - 1
)

const (
	// Bold flag for ColorScheme.
	bold     color = 1<<bitsBold | noColor
	bitsBold       = 8
	maskBold       = 1 << bitsBold

	bright                     color = 1<<bitsBright | noColor
	bitsBright                       = 9
	maskBright                       = 1 << bitsBright
	ansiBrightForegroundOffset       = 90 - 1

	backgroundBright           color = 1<<bitsBackgroundBright | noColor
	bitsBackgroundBright             = 10
	maskBackgroundBright             = 1 << bitsBackgroundBright
	ansiBrightBackgroundOffset       = 100 - 1
)

const ()

func colorize(text string, color color) string {
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
