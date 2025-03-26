package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/kellegous/glue/build"
)

func main() {
	// TODO(kellegous): Add the ability to get individual properties via templating.
	summary, err := build.ReadSummary()
	if err != nil {
		log.Fatal(err)
	}

	b, err := json.Marshal(summary)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%s\n", b)
}
