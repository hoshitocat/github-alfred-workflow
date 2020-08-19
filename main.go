package main

import (
	"encoding/json"
	"fmt"

	aw "github.com/deanishe/awgo"
)

var wf *aw.Workflow

func init() {
	wf = aw.New()
}

func auth() {
	alfredVars := map[string]string{"email": "test@test.com"}
	b, _ := json.Marshal(map[string]interface{}{"alfredworkflow": map[string]interface{}{"variables": alfredVars}})
	fmt.Println(string(b))
}

func run() {
	args := wf.Args()
	if len(args) == 0 {
		wf.Fatal("invalid command arguments")
		return
	}

	commandOperator := args[0]

	switch commandOperator {
	case "auth":
		auth()
	}

	// wf.NewItem(query)
	// wf.WarnEmpty("Can't find repository or user", "Try a different query?")
	// wf.SendFeedback()
}

func main() {
	wf.Run(run)
}
