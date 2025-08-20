package main

import (
	"fmt"
	"log"

	"github.com/xtdlib/try"
)

type Catchable interface {
	Catch() error
}

func main() {
	log.Println("start")
	err := failfunc()
	if err != nil {
		log.Printf("caught error: %v", err)
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

func err1() (err error) {
	return fmt.Errorf("err1: error")
}
