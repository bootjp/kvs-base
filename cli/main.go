package main

import (
	"log"

	"github.com/bootjp/kvs-infrastructure"
)

func main() {
	err := kvs.Run()
	if err != nil {
		log.Fatal(err)
	}
}
