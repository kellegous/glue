package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"log"

	"github.com/kellegous/glue/build"
)

func main() {
	var encode bool
	flag.BoolVar(&encode, "encode", false, "output the summary as a base64 encoded string")
	flag.Parse()

	s, err := build.ReadSummaryFromGit(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	b, err := json.Marshal(s)
	if err != nil {
		log.Fatal(err)
	}

	if encode {
		fmt.Printf("%s\n", base64.StdEncoding.EncodeToString(b))
	} else {
		fmt.Printf("%s\n", b)
	}
}
