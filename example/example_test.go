package example

import (
	"os"
	"testing"

	"github.com/xtdlib/try"
)

func TestExample(t *testing.T) {
	defer try.TestFatal(t)
	try.E1(os.Open("nonexistent"))
}
