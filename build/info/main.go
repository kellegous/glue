package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"text/template"

	"github.com/kellegous/glue/build"
)

func applyFormat(s *build.Summary, format string) ([]byte, error) {
	tpl, err := template.New("format").Parse(format)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer

	params := map[string]any{
		"SHA":         s.SHA,
		"CommitTime":  s.CommitTime,
		"Name":        s.Name,
		"sha":         s.SHA,
		"commit_time": s.CommitTime,
		"name":        s.Name,
	}

	if err := tpl.Execute(&buf, params); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func main() {
	var encode bool
	var format string
	flag.BoolVar(
		&encode,
		"encode",
		false,
		"output the summary as a base64 encoded string")
	flag.StringVar(
		&format,
		"format",
		"",
		"a text/template expression to format the output")
	flag.Parse()

	s, err := build.ReadSummaryFromGit(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	var out []byte
	if format != "" {
		out, err = applyFormat(s, format)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		out, err = json.Marshal(s)
		if err != nil {
			log.Fatal(err)
		}
	}

	if encode {
		fmt.Printf("%s\n", base64.StdEncoding.EncodeToString(out))
	} else {
		fmt.Printf("%s\n", out)
	}
}
