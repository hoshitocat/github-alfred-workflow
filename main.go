package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	aw "github.com/deanishe/awgo"
	github "github.com/shurcooL/githubv4"

	"golang.org/x/oauth2"
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

type AuthUser struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Token string `json:"token"`
	URL   string `json:"url"`
}

func auth(token string) {
	ctx := context.Background()
	src := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	httpClient := oauth2.NewClient(ctx, src)
	client := github.NewClient(httpClient)

	var query struct {
		Viewer struct {
			Login github.String
			Email github.String
			Url   github.URI
		}
	}
	err := client.Query(ctx, &query, nil)
	if err != nil {
		wf.FatalError(err)
		return
	}

	err = os.MkdirAll(os.Getenv("HOME")+"/.config/github-alfred-workflow", 0755)
	if err != nil {
		wf.FatalError(err)
		return
	}

	fp, err := os.Create(os.Getenv("HOME") + "/.config/github-alfred-workflow/credentials.json")
	if err != nil {
		wf.FatalError(err)
		return
	}

	authUser := AuthUser{
		Name:  string(query.Viewer.Login),
		Email: string(query.Viewer.Email),
		Token: token,
		URL:   query.Viewer.Url.String(),
	}
	credentials, err := json.Marshal(authUser)
	if err != nil {
		wf.FatalError(err)
		return
	}

	_, err = fp.Write(credentials)
	if err != nil {
		wf.FatalError(err)
		return
	}

	fmt.Println(query)

	// vars := AuthVariables{"hoshitocat", "hoshitocat@gmail.com"}
	// resp := Response{AlfredWorkflow{Variables: vars}}
	// b, _ := json.Marshal(resp)
	// fmt.Println(string(b))
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
