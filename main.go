package main

import (
	aw "github.com/deanishe/awgo"
	"log"
)

var wf *aw.Workflow

func init() {
	wf = aw.New()
}

func run() {
	var query string

	if args := wf.Args(); len(args) > 0 {
		query = args[0]
	}

	log.Printf("run: query=%s\n", query)
	wf.NewItem(query)
	wf.WarnEmpty("Can't find repository or user", "Try a different query?")
	wf.SendFeedback()
}

func main() {
	wf.Run(run)
}
