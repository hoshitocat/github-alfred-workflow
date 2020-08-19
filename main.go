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

type Response struct {
	AlfredWorkflow `json:"alfredworkflow"`
}

type AlfredWorkflow struct {
	Variables interface{} `json:"variables"`
}

type AuthVariables struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func auth(token string) {
	vars := AuthVariables{"hoshitocat", "hoshitocat@gmail.com"}
	resp := Response{AlfredWorkflow{Variables: vars}}
	b, _ := json.Marshal(resp)
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
		token := args[1]
		auth(token)
	}

	// wf.NewItem(query)
	// wf.WarnEmpty("Can't find repository or user", "Try a different query?")
	// wf.SendFeedback()
}

func main() {
	wf.Run(run)
}
