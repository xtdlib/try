package main

import (
	"os"

	"github.com/xtdlib/try"
)

func main() {
	try.E1(os.Open("not_exist_file"), nil)
}
