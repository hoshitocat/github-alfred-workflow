package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"

	aw "github.com/deanishe/awgo"
	github "github.com/shurcooL/githubv4"

	"golang.org/x/oauth2"
)

var (
	wf          *aw.Workflow
	credFileDir = os.Getenv("HOME") + "/.config/github-alfred-workflow"
	credFile    = credFileDir + "/credentials.json"
)

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

type AuthResponse struct {
	Title string `json:"title"`
	Text  string `json:"text"`
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
		resp := AuthResponse{
			Title: "Authentication Failed",
			Text:  err.Error(),
		}
		b, e := json.Marshal(Response{AlfredWorkflow{Variables: resp}})
		if e != nil {
			wf.FatalError(err)
		}
		fmt.Println(string(b))
		return
	}

	err = os.MkdirAll(credFileDir, 0755)
	if err != nil {
		resp := AuthResponse{
			Title: "Authentication Failed",
			Text:  err.Error(),
		}
		b, e := json.Marshal(Response{AlfredWorkflow{Variables: resp}})
		if e != nil {
			wf.FatalError(err)
		}
		fmt.Println(string(b))
		return
	}

	fp, err := os.Create(credFile)
	if err != nil {
		resp := AuthResponse{
			Title: "Authentication Failed",
			Text:  err.Error(),
		}
		b, e := json.Marshal(Response{AlfredWorkflow{Variables: resp}})
		if e != nil {
			wf.FatalError(err)
		}
		fmt.Println(string(b))
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
		resp := AuthResponse{
			Title: "Authentication Failed",
			Text:  err.Error(),
		}
		b, e := json.Marshal(Response{AlfredWorkflow{Variables: resp}})
		if e != nil {
			wf.FatalError(err)
		}
		fmt.Println(string(b))
		return
	}

	_, err = fp.Write(credentials)
	if err != nil {
		resp := AuthResponse{
			Title: "Authentication Failed",
			Text:  err.Error(),
		}
		b, e := json.Marshal(Response{AlfredWorkflow{Variables: resp}})
		if e != nil {
			wf.FatalError(err)
		}
		fmt.Println(string(b))
		return
	}

	resp := AuthResponse{
		Title: "Authentication Succeeded",
		Text:  fmt.Sprintf("Hello, %s", authUser.Name),
	}
	b, e := json.Marshal(Response{AlfredWorkflow{Variables: resp}})
	if e != nil {
		wf.FatalError(err)
	}
	fmt.Println(string(b))
}

func search(name string) {
	b, err := ioutil.ReadFile(credFile)
	if err != nil {
		resp := AuthResponse{
			Title: "Authentication Failed",
			Text:  err.Error(),
		}
		b, e := json.Marshal(Response{AlfredWorkflow{Variables: resp}})
		if e != nil {
			wf.FatalError(err)
		}
		fmt.Println(string(b))
		return
	}

	var authUser AuthUser
	err = json.Unmarshal(b, &authUser)
	if err != nil {
		resp := AuthResponse{
			Title: "Authentication Failed",
			Text:  err.Error(),
		}
		b, e := json.Marshal(Response{AlfredWorkflow{Variables: resp}})
		if e != nil {
			wf.FatalError(err)
		}
		fmt.Println(string(b))
		return
	}

	ctx := context.Background()
	src := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: authUser.Token})
	httpClient := oauth2.NewClient(ctx, src)
	client := github.NewClient(httpClient)

	var query struct {
		Search struct {
			PageInfo struct {
				StartCursor github.String
			}
			Edges []struct {
				Cursor github.String
				Node   struct {
					Repository struct {
						NameWithOwner github.String
						URL           github.URI
					} `graphql:"... on Repository"`
				}
			}
		} `graphql:"search(query: $name, type: REPOSITORY, first: 100)"`
	}

	err = client.Query(ctx, &query, map[string]interface{}{"name": github.String(fmt.Sprintf("%s in:name", name))})
	if err != nil {
		wf.FatalError(err)
		return
	}

	for _, repo := range query.Search.Edges {
		r := repo.Node.Repository
		name, url := string(r.NameWithOwner), r.URL.String()
		j, err := json.Marshal(map[string]string{"name": name, "url": url})
		if err != nil {
			wf.FatalError(err)
			return
		}
		wf.NewItem(name).Autocomplete(name).Arg(string(j)).Valid(true)
	}

	wf.SendFeedback()
}

type Repository struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

func action(operation string) {
	b, err := ioutil.ReadFile("./repository.json")
	if err != nil {
		wf.FatalError(err)
	}

	repo := Repository{}
	err = json.Unmarshal(b, &repo)
	if err != nil {
		wf.FatalError(err)
	}

	url := repo.URL
	switch operation {
	case "pulls":
		url += "/pulls"
	case "issues":
		url += "/issues"
	}

	err = exec.Command("open", url).Start()
	if err != nil {
		wf.FatalError(err)
		return
	}
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
	case "search":
		var name string
		if len(args) > 1 {
			name = args[1]
		}
		search(name)
	case "action":
		action(args[1])
	}
}

func main() {
	wf.Run(run)
}
