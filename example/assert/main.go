package main

import (
	"github.com/xtdlib/try"
)

func main() {
	var v any
	// try.Zero(v)
	try.NotZero(v)
}
