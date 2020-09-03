package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os/exec"
	"strings"
	"time"

	aw "github.com/deanishe/awgo"
	github "github.com/shurcooL/githubv4"

	"golang.org/x/oauth2"
)

const (
	cacheKeyAuth         = "auth.json"
	cacheKeyRepositories = "repositories.json"
	cacheMaxAge          = time.Hour * 24 * 30
)

var (
	wf         *aw.Workflow
	ErrNoCache = errors.New("no cache")
)

func init() {
	wf = aw.New()
}

type PageInfo struct {
	StartCursor string `json:"start_cursor"`
	EndCursor   string `json:"end_cursor"`
	HasNextPage bool   `json:"has_next_page"`
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

type Repository struct {
	Name string `json:"name"`
	URL  string `json:"url"`
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

	authUser := AuthUser{
		Name:  string(query.Viewer.Login),
		Email: string(query.Viewer.Email),
		Token: token,
		URL:   query.Viewer.Url.String(),
	}
	err = wf.Cache.StoreJSON(cacheKeyAuth, authUser)
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

func fetchRepositoriesFromCache() ([]*Repository, error) {
	if !wf.Cache.Exists(cacheKeyRepositories) {
		return nil, ErrNoCache
	}

	var repositories struct {
		Repositories []*Repository `json:"repositories"`
	}
	err := wf.Cache.LoadJSON(cacheKeyRepositories, &repositories)
	if err != nil {
		return nil, err
	}

	return repositories.Repositories, nil
}

func fetchOwnRepositories(ctx context.Context, client *github.Client, after string) ([]*Repository, *PageInfo, error) {
	var ownRepositoriesQuery struct {
		Viewer struct {
			Repositories struct {
				PageInfo struct {
					StartCursor github.String
					EndCursor   github.String
					HasNextPage github.Boolean
				}
				Edges []struct {
					Node struct {
						Repository struct {
							NameWithOwner github.String
							URL           github.URI
						} `graphql:"... on Repository"`
					}
				}
			} `graphql:"repositories(first: 100, after: $after, affiliations:[OWNER, COLLABORATOR, ORGANIZATION_MEMBER], ownerAffiliations:[OWNER, ORGANIZATION_MEMBER, COLLABORATOR])"`
		}
	}

	var err error
	if after == "" {
		err = client.Query(ctx, &ownRepositoriesQuery, map[string]interface{}{"after": (*github.String)(nil)})
	} else {
		err = client.Query(ctx, &ownRepositoriesQuery, map[string]interface{}{"after": github.String(after)})
	}
	if err != nil {
		return nil, nil, err
	}

	var repositories []*Repository
	for _, repo := range ownRepositoriesQuery.Viewer.Repositories.Edges {
		repositories = append(repositories, &Repository{
			Name: string(repo.Node.Repository.NameWithOwner),
			URL:  repo.Node.Repository.URL.String(),
		})
	}

	return repositories, &PageInfo{
		StartCursor: string(ownRepositoriesQuery.Viewer.Repositories.PageInfo.StartCursor),
		EndCursor:   string(ownRepositoriesQuery.Viewer.Repositories.PageInfo.EndCursor),
		HasNextPage: bool(ownRepositoriesQuery.Viewer.Repositories.PageInfo.HasNextPage),
	}, nil
}

func fetchOwnAllRepositories(ctx context.Context, client *github.Client) ([]*Repository, error) {
	var repositories []*Repository
	var pageAfter string
	for {
		r, pageInfo, err := fetchOwnRepositories(ctx, client, pageAfter)
		if err != nil {
			return nil, err
		}

		repositories = append(repositories, r...)

		if !pageInfo.HasNextPage {
			break
		}

		pageAfter = pageInfo.EndCursor
	}

	return repositories, nil
}

func cacheRepositories(repositories []*Repository) error {
	j, err := json.Marshal(map[string]interface{}{"repositories": repositories})
	if err != nil {
		return err
	}

	wf.Cache.Expired(cacheKeyRepositories, cacheMaxAge)
	err = wf.Cache.Store(cacheKeyRepositories, j)
	if err != nil {
		return err
	}

	return nil
}

func search(searchQuery string) {
	var authUser AuthUser
	err := wf.Cache.LoadJSON(cacheKeyAuth, &authUser)
	if err != nil {
		wf.FatalError(err)
		return
	}

	ctx := context.Background()
	src := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: authUser.Token})
	httpClient := oauth2.NewClient(ctx, src)
	client := github.NewClient(httpClient)

	var repositories []*Repository
	repositories, err = fetchRepositoriesFromCache()
	if err == ErrNoCache {
		var e error
		repositories, e = fetchOwnAllRepositories(ctx, client)
		if e != nil {
			wf.FatalError(e)
			return
		}

		e = cacheRepositories(repositories)
		if e != nil {
			wf.FatalError(e)
			return
		}

		feedbackRepositories(repositories, searchQuery)
	} else {
		if err != nil {
			wf.FatalError(err)
			return
		}

		if !feedbackRepositories(repositories, searchQuery) {
			var e error
			repositories, e = fetchOwnAllRepositories(ctx, client)
			if e != nil {
				wf.FatalError(err)
				return
			}

			e = cacheRepositories(repositories)
			if e != nil {
				wf.FatalError(e)
				return
			}

			feedbackRepositories(repositories, searchQuery)
		}
	}

	wf.SendFeedback()
}

func feedbackRepositories(repositories []*Repository, query string) bool {
	for _, r := range repositories {
		if !strings.Contains(r.Name, query) {
			continue
		}

		b, err := json.Marshal(r)
		if err != nil {
			wf.FatalError(err)
			return false
		}

		wf.NewItem(r.Name).Autocomplete(r.Name).Arg(string(b)).Valid(true)
	}

	if len(wf.Feedback.Items) == 0 {
		return false
	}

	return true
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
