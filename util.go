package main

import "strings"

func splitStringOnceRune(value string, delimiter rune) (a, b string) {
	const notFoundIndex = -1
	delimiterIndex := strings.IndexRune(value, delimiter)
	if delimiterIndex == notFoundIndex {
		a = value
		b = ""
		return
	}
	a = value[:delimiterIndex]
	b = value[delimiterIndex+1:] // +1 to skip the delimiter
	return
}
