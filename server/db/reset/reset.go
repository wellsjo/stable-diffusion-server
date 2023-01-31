package main

import (
	"log"
	"os"

	"github.com/juju/errors"
	"github.com/wellsjo/ai-art/server/db"
)

func main() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Lshortfile)

	db, err := db.GetTestConnection()
	if err != nil {
		log.Fatal(errors.ErrorStack(err))
	}
	err = db.Reset()
	if err != nil {
		log.Fatal(errors.ErrorStack(err))
	}
	log.Println("Done")
}
