package main

import (
	"fmt"
	"log"
	"os"

	"log/slog"

	"github.com/xtdlib/try"
)

type Catchable interface {
	Catch() error
}

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))
	log.Println("start")
	err := failfunc()
	if err != nil {
		log.Print(err)
		log.Print(err == Err1)
	}
	log.Println("done")
}

func failfunc() (ferr error) {
	defer try.Catch(&ferr)
	// defer try.Catch(&ferr)
	fpanic()
	// panic("this is a panic")
	return
}

func fpanic() {
	try.E(err1())
	return
}

var Err1 = fmt.Errorf("Err1")

func err1() (err error) {
	return Err1
}
