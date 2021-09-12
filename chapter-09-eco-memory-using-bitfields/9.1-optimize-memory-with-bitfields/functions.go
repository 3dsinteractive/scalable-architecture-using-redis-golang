package main

import (
	"strings"

	"github.com/segmentio/ksuid"
)

// RemoveTabAndNewLine remove tab and new line from string
func RemoveTabAndNewLine(str string) string {
	return strings.Replace(strings.Replace(str, "	", "", -1), "\n", "", -1)
}

// NewUUID return new UUID as string
func NewUUID() string {
	id := ksuid.New()
	return id.String()
}
