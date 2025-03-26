package main

import (
	"context"
	"fmt"
	"log"

	"github.com/kellegous/glue/build"
)

func main() {
	info, err := build.VCSInfoFromGit(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(info)
}
