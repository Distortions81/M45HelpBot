package sclean

import (
	"fmt"
	"regexp"
	"strings"
)

func AlphaOnly(str string) string {
	alphafilter, _ := regexp.Compile("[^a-zA-Z]+")
	str = alphafilter.ReplaceAllString(str, "")
	return str
}

func NumOnly(str string) string {
	alphafilter, _ := regexp.Compile("[^0-9]+")
	str = alphafilter.ReplaceAllString(str, "")
	return str
}

func AlphaNumOnly(str string) string {
	alphafilter, _ := regexp.Compile("[^a-zA-Z0-9]+")
	str = alphafilter.ReplaceAllString(str, "")
	return str
}

/* Strip all but a-z */
func StripControlAndSpecial(str string) string {
	b := make([]byte, len(str))
	var bl int
	for i := 0; i < len(str); i++ {
		c := str[i]
		if c >= 32 && c < 127 {
			b[bl] = c
			bl++
		}
	}
	return string(b[:bl])
}

/* Strip lower ascii codes, sub newlines, returns and tabs with ' ' */
func StripControlAndSubSpecial(str string) string {
	b := make([]byte, len(str))
	var bl int
	for i := 0; i < len(str); i++ {
		c := str[i]
		if c == '\n' || c == '\r' || c == '\t' {
			b[bl] = ' '
			bl++
		} else if c >= 32 && c != 127 {
			b[bl] = c
			bl++
		}
	}
	return string(b[:bl])
}

/* Strip lower ascii codes */
func StripControl(str string) string {
	b := make([]byte, len(str))
	var bl int
	for i := 0; i < len(str); i++ {
		c := fmt.Sprintf("%c", i)
		if c[0] >= 32 && c[0] != 127 {
			b[bl] = c[0]
			bl++
		}
	}
	return string(b[:bl])
}

func RemoveDiscordMarkdown(input string) string {
	/* Remove Discord markdown */
	regf := regexp.MustCompile(`\*+`)
	regg := regexp.MustCompile(`\~+`)
	regh := regexp.MustCompile(`\_+`)
	for regf.MatchString(input) || regg.MatchString(input) || regh.MatchString(input) {
		/* Filter Discord tags */
		input = regf.ReplaceAllString(input, "")
		input = regg.ReplaceAllString(input, "")
		input = regh.ReplaceAllString(input, "")
		input = strings.ReplaceAll(input, "`", "")
	}

	return input
}
