package zsxq

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go exposes zsxq as a kit Domain. A multi-domain host enables it
// with a single blank import:
//
//	import _ "github.com/tamnd/zsxq-cli/zsxq"
//
// The same Domain builds the standalone zsxq binary via cli.NewApp.
func init() { kit.Register(Domain{}) }

// Domain is the Zsxq driver. It carries no state; the per-run client is
// built by the factory Register hands to kit.
type Domain struct{}

// Info describes the scheme and identity that the binary inherits.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "zsxq",
		Hosts:  []string{Host, "wx." + Host, "api." + Host},
		Identity: kit.Identity{
			Binary: "zsxq",
			Short:  "Read Zsxq knowledge group data",
			Long: `zsxq turns wx.zsxq.com into a fast, scriptable command line.

All commands require ZSXQ_ACCESS_TOKEN. To get it: log in at
https://wx.zsxq.com/, open browser dev tools, go to
Application > Cookies > wx.zsxq.com, and copy zsxq_access_token.

Quick start:
  zsxq explore                          list public knowledge groups
  zsxq explore --category tech          tech groups only
  zsxq group 28855514824481             fetch a group by ID
  zsxq search "golang"                  search groups
  zsxq topic 18885512814151             fetch a topic by ID
  zsxq trending                         trending hashtags`,
			Site: Host,
			Repo: "https://github.com/tamnd/zsxq-cli",
		},
	}
}

// Register installs the client factory and all operations onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{
		Name:    "explore",
		Group:   "browse",
		Summary: "List public knowledge groups",
	}, exploreGroups)

	kit.Handle(app, kit.OpMeta{
		Name:     "group",
		Group:    "browse",
		Single:   true,
		Resolver: true,
		URIType:  "group",
		Summary:  "Fetch a knowledge group by ID",
		Args:     []kit.Arg{{Name: "id", Help: "group ID"}},
	}, getGroup)

	kit.Handle(app, kit.OpMeta{
		Name:    "search",
		Group:   "browse",
		Summary: "Search for knowledge groups",
		Args:    []kit.Arg{{Name: "query", Help: "search keyword"}},
	}, searchGroups)

	kit.Handle(app, kit.OpMeta{
		Name:     "topic",
		Group:    "browse",
		Single:   true,
		Resolver: true,
		URIType:  "topic",
		Summary:  "Fetch a topic by ID",
		Args:     []kit.Arg{{Name: "id", Help: "topic ID"}},
	}, getTopic)

	kit.Handle(app, kit.OpMeta{
		Name:    "trending",
		Group:   "browse",
		Summary: "List trending hashtags",
	}, getTrending)
}

// newClient builds a Client from the kit Config and environment.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := DefaultConfig()
	c.AccessToken = os.Getenv("ZSXQ_ACCESS_TOKEN")
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.Timeout = cfg.Timeout
	}
	return NewClient(c), nil
}

// --- input structs ---

type exploreInput struct {
	Category string  `kit:"flag" help:"category slug (tech, finance, design, health, career, culture, hobby)"`
	Limit    int     `kit:"flag,inherit" help:"max groups" default:"20"`
	Client   *Client `kit:"inject"`
}

type groupInput struct {
	ID     string  `kit:"arg" help:"group ID"`
	Topics int     `kit:"flag" help:"also emit recent topics (0 = no topics)" default:"0"`
	Client *Client `kit:"inject"`
}

type searchInput struct {
	Query  string  `kit:"arg" help:"search keyword"`
	Limit  int     `kit:"flag,inherit" help:"max results" default:"10"`
	Client *Client `kit:"inject"`
}

type topicInput struct {
	ID     string  `kit:"arg" help:"topic ID"`
	Client *Client `kit:"inject"`
}

type trendingInput struct {
	Limit  int     `kit:"flag,inherit" help:"max hashtags" default:"20"`
	Client *Client `kit:"inject"`
}

// --- handlers ---

func exploreGroups(ctx context.Context, in exploreInput, emit func(Group) error) error {
	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}
	groups, err := in.Client.ExploreGroups(ctx, in.Category, limit)
	if err != nil {
		return mapErr(err)
	}
	for _, g := range groups {
		if err := emit(g); err != nil {
			return err
		}
	}
	return nil
}

func getGroup(ctx context.Context, in groupInput, emit func(any) error) error {
	g, err := in.Client.GetGroup(ctx, in.ID)
	if err != nil {
		return mapErr(err)
	}
	if err := emit(g); err != nil {
		return err
	}
	if in.Topics > 0 {
		topics, err := in.Client.GetGroupTopics(ctx, in.ID, in.Topics)
		if err != nil {
			return mapErr(err)
		}
		for _, t := range topics {
			if err := emit(t); err != nil {
				return err
			}
		}
	}
	return nil
}

func searchGroups(ctx context.Context, in searchInput, emit func(SearchResult) error) error {
	limit := in.Limit
	if limit <= 0 {
		limit = 10
	}
	results, err := in.Client.SearchGroups(ctx, in.Query, limit)
	if err != nil {
		return mapErr(err)
	}
	for _, r := range results {
		if err := emit(r); err != nil {
			return err
		}
	}
	return nil
}

func getTopic(ctx context.Context, in topicInput, emit func(*Topic) error) error {
	t, err := in.Client.GetTopic(ctx, in.ID)
	if err != nil {
		return mapErr(err)
	}
	return emit(t)
}

func getTrending(ctx context.Context, in trendingInput, emit func(Hashtag) error) error {
	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}
	hashtags, err := in.Client.GetTrendingHashtags(ctx, limit)
	if err != nil {
		return mapErr(err)
	}
	for _, h := range hashtags {
		if err := emit(h); err != nil {
			return err
		}
	}
	return nil
}

// --- Resolver ---

// Classify turns a Zsxq URL or ID into (uriType, id).
func (Domain) Classify(input string) (uriType, id string, err error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", "", errs.Usage("zsxq: empty input")
	}
	// wx.zsxq.com/group/<id> or wx.zsxq.com/topic/<id>
	if strings.Contains(input, "zsxq.com") {
		parts := strings.Split(strings.Trim(input, "/"), "/")
		if len(parts) >= 2 {
			uriType = parts[len(parts)-2]
			id = parts[len(parts)-1]
			if uriType == "group" || uriType == "topic" {
				return uriType, id, nil
			}
		}
	}
	// Bare numeric ID: treat as group.
	return "group", input, nil
}

// Locate returns the canonical URL for a (uriType, id).
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "group":
		return WebBase + "/group/" + id, nil
	case "topic":
		return WebBase + "/topic/" + id, nil
	default:
		return "", errs.Usage("zsxq has no resource type %q", uriType)
	}
}

// mapErr maps library sentinel errors to kit error kinds.
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrNoAuth) {
		return errs.NeedAuth("%s", err.Error())
	}
	if errors.Is(err, ErrNotFound) {
		return errs.NotFound("%s", err.Error())
	}
	return err
}
