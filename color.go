package main

import (
	"log"

	"fmt"
)
import "strings"

const (
	colorReset = 0
	colorRed   = 31
	colorGreen = 32
	colorBlue  = 34
)

var nameToColor = map[string]int{
	"red":   colorRed,
	"green": colorGreen,
	"blue":  colorBlue,
}

func colorize(s string, color int) string {
	return fmt.Sprintf("\x1b[%d;1m%s\x1b[%dm", color, s, colorReset)
}

func Csprintf(format string, args ...interface{}) string {
	for name, color := range nameToColor {
		startTag := fmt.Sprintf("#%s{", name)
	loop:
		for start := strings.Index(format, startTag); start >= 0; start = strings.Index(format, startTag) {
			contentsStart := start + len(startTag)
			for i := contentsStart; i < len(format); i++ {
				if format[i] == '}' {
					format = format[:start] + colorize(format[contentsStart:i], color) + format[i+1:]
					continue loop
				}
			}
			return "< !BAD COLOR FORMAT -- NO CLOSING } >"
		}
	}
	return fmt.Sprintf(format, args...)
}

func LogCprintf(format string, args ...interface{}) { log.Print(Csprintf(format, args...)) }
