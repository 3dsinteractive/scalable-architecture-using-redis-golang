package main

import (
	"math/rand"
	"strings"
	"time"

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

func RandomMinMax(min int, max int) int {
	s := rand.NewSource(time.Now().UnixNano())
	r := rand.New(s)
	return r.Intn(max-min+1) + min
}
