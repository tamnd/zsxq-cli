// Package zsxq is the library behind the zsxq command: the HTTP client,
// cookie-based auth, pacing, and typed data models for Zsxq (知识星球).
//
// All Zsxq API endpoints require authentication via the zsxq_access_token
// cookie. Set ZSXQ_ACCESS_TOKEN to your cookie value. Without it, every
// method returns ErrNoAuth so the caller can surface exit code 5.
//
// Get your token by logging in at https://wx.zsxq.com/, opening browser
// dev tools, going to Application > Cookies > wx.zsxq.com, and copying
// the value of zsxq_access_token.
package zsxq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Host is the main Zsxq web hostname.
const Host = "zsxq.com"

// APIBase is the root for all Zsxq API requests.
const APIBase = "https://api." + Host + "/v2"

// WebBase is the base URL for building user-facing links.
const WebBase = "https://wx." + Host

// DefaultUserAgent mimics a browser session as Zsxq expects.
const DefaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// ErrNoAuth is returned when ZSXQ_ACCESS_TOKEN is not set or the API
// returns 401. The caller should map this to exit code 5.
var ErrNoAuth = errors.New("zsxq: authentication required; set ZSXQ_ACCESS_TOKEN (see: zsxq-cli.tamnd.com/auth)")

// ErrNotFound is returned when the API returns a 404 or the envelope
// indicates the resource does not exist.
var ErrNotFound = errors.New("not found")

// Config holds constructor parameters for Client.
type Config struct {
	AccessToken string
	UserAgent   string
	Rate        time.Duration
	Retries     int
	Timeout     time.Duration
}

// DefaultConfig returns sensible defaults for the Zsxq client.
func DefaultConfig() Config {
	return Config{
		UserAgent: DefaultUserAgent,
		Rate:      300 * time.Millisecond,
		Retries:   3,
		Timeout:   30 * time.Second,
	}
}

// envelope is the standard Zsxq API response wrapper.
type envelope struct {
	Succeeded bool            `json:"succeeded"`
	Code      int             `json:"code"`
	Error     string          `json:"error"`
	Info      string          `json:"info"`
	RespData  json.RawMessage `json:"resp_data"`
}

// Client is a rate-limited HTTP client for the Zsxq API.
type Client struct {
	cfg  Config
	http *http.Client
	last time.Time
}

// NewClient returns a Client configured with cfg.
// If cfg.AccessToken is empty, all API methods return ErrNoAuth immediately.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout},
	}
}

// checkAuth returns ErrNoAuth if no token is set.
func (c *Client) checkAuth() error {
	if c.cfg.AccessToken == "" {
		return ErrNoAuth
	}
	return nil
}

// get fetches an API path, decodes the envelope, and returns resp_data.
func (c *Client) get(ctx context.Context, path string, query url.Values) (json.RawMessage, error) {
	if err := c.checkAuth(); err != nil {
		return nil, err
	}

	rawURL := APIBase + path
	if len(query) > 0 {
		rawURL += "?" + query.Encode()
	}

	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			wait := time.Duration(attempt) * 500 * time.Millisecond
			if wait > 5*time.Second {
				wait = 5 * time.Second
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wait):
			}
		}
		data, retry, err := c.do(ctx, rawURL)
		if err == nil {
			return data, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", path, lastErr)
}

func (c *Client) do(ctx context.Context, rawURL string) (json.RawMessage, bool, error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Cookie", "zsxq_access_token="+c.cfg.AccessToken)
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Referer", "https://wx.zsxq.com/")
	req.Header.Set("Origin", "https://wx.zsxq.com")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, true, err
	}

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}

	// Parse the Zsxq response envelope.
	var env envelope
	if err := json.Unmarshal(b, &env); err != nil {
		if resp.StatusCode != http.StatusOK {
			return nil, false, fmt.Errorf("http %d", resp.StatusCode)
		}
		return nil, false, fmt.Errorf("decode response: %w", err)
	}

	if !env.Succeeded {
		if env.Code == 401 || env.Code == 403 {
			return nil, false, ErrNoAuth
		}
		if env.Code == 404 {
			return nil, false, ErrNotFound
		}
		return nil, false, fmt.Errorf("api error %d: %s", env.Code, env.Info)
	}

	return env.RespData, false, nil
}

// pace sleeps until at least Rate has elapsed since the last request.
func (c *Client) pace() {
	if c.cfg.Rate <= 0 {
		return
	}
	if wait := c.cfg.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

// ExploreGroups fetches the public group explore listing.
func (c *Client) ExploreGroups(ctx context.Context, category string, limit int) ([]Group, error) {
	catID := categoryID(category)
	q := url.Values{
		"count":       {fmt.Sprintf("%d", limit)},
		"category_id": {catID},
	}
	data, err := c.get(ctx, "/groups/explore", q)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Groups []rawGroup `json:"groups"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode groups: %w", err)
	}
	out := make([]Group, len(resp.Groups))
	for i, g := range resp.Groups {
		out[i] = groupFromRaw(g)
	}
	return out, nil
}

// GetGroup fetches a single group by ID.
func (c *Client) GetGroup(ctx context.Context, id string) (*Group, error) {
	data, err := c.get(ctx, "/groups/"+id, nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Group rawGroup `json:"group"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode group: %w", err)
	}
	g := groupFromRaw(resp.Group)
	return &g, nil
}

// GetGroupTopics fetches recent topics for a group.
func (c *Client) GetGroupTopics(ctx context.Context, groupID string, limit int) ([]Topic, error) {
	q := url.Values{
		"count": {fmt.Sprintf("%d", limit)},
	}
	data, err := c.get(ctx, "/groups/"+groupID+"/topics", q)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Topics []rawTopic `json:"topics"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode topics: %w", err)
	}
	out := make([]Topic, len(resp.Topics))
	for i, t := range resp.Topics {
		out[i] = topicFromRaw(t)
	}
	return out, nil
}

// SearchGroups searches for groups by keyword.
func (c *Client) SearchGroups(ctx context.Context, keyword string, limit int) ([]SearchResult, error) {
	q := url.Values{
		"keyword": {keyword},
		"count":   {fmt.Sprintf("%d", limit)},
	}
	data, err := c.get(ctx, "/search/groups", q)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Groups []rawGroup `json:"groups"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode search: %w", err)
	}
	out := make([]SearchResult, len(resp.Groups))
	for i, g := range resp.Groups {
		gr := groupFromRaw(g)
		out[i] = SearchResult{
			ID:          gr.ID,
			Name:        gr.Name,
			Type:        "group",
			Description: gr.Description,
			MemberCount: gr.MemberCount,
			URL:         gr.URL,
		}
	}
	return out, nil
}

// GetTopic fetches a single topic by ID.
func (c *Client) GetTopic(ctx context.Context, id string) (*Topic, error) {
	data, err := c.get(ctx, "/topics/"+id, nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Topic rawTopic `json:"topic"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode topic: %w", err)
	}
	t := topicFromRaw(resp.Topic)
	return &t, nil
}

// GetTrendingHashtags fetches trending hashtags.
func (c *Client) GetTrendingHashtags(ctx context.Context, limit int) ([]Hashtag, error) {
	q := url.Values{
		"count": {fmt.Sprintf("%d", limit)},
	}
	data, err := c.get(ctx, "/hashtags/trending", q)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Hashtags []rawHashtag `json:"hashtags"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode hashtags: %w", err)
	}
	out := make([]Hashtag, len(resp.Hashtags))
	for i, h := range resp.Hashtags {
		out[i] = hashtagFromRaw(h)
	}
	return out, nil
}

// --- raw API types ---

type rawGroup struct {
	GroupID     string      `json:"group_id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Owner       rawUser     `json:"owner"`
	MemberCount int         `json:"member_count"`
	TopicCount  int         `json:"topic_count"`
	CreatedTime string      `json:"created_time"`
	Price       interface{} `json:"price"`
	Currency    string      `json:"currency"`
	Category    struct {
		Name string `json:"name"`
	} `json:"category"`
}

type rawUser struct {
	UserID string `json:"user_id"`
	Name   string `json:"name"`
}

type rawTopic struct {
	TopicID       string   `json:"topic_id"`
	Group         rawGroup `json:"group"`
	Type          string   `json:"type"`
	Owner         rawUser  `json:"owner"`
	Talk          struct {
		Text string `json:"text"`
	} `json:"talk"`
	LikesCount    int    `json:"likes_count"`
	CommentsCount int    `json:"comments_count"`
	CreateTime    string `json:"create_time"`
}

type rawHashtag struct {
	HashtagID  string `json:"hashtag_id"`
	Name       string `json:"name"`
	TopicCount int    `json:"topic_count"`
}

// --- converters ---

func groupFromRaw(g rawGroup) Group {
	// price may be int or float from JSON.
	var price float64
	switch v := g.Price.(type) {
	case float64:
		price = v
	case int:
		price = float64(v)
	}
	currency := g.Currency
	if currency == "" {
		currency = "CNY"
	}
	return Group{
		ID:          g.GroupID,
		Name:        g.Name,
		Description: g.Description,
		OwnerName:   g.Owner.Name,
		OwnerID:     g.Owner.UserID,
		MemberCount: g.MemberCount,
		TopicCount:  g.TopicCount,
		CreatedAt:   g.CreatedTime,
		Price:       price,
		Currency:    currency,
		Category:    g.Category.Name,
		URL:         WebBase + "/group/" + g.GroupID,
	}
}

func topicFromRaw(t rawTopic) Topic {
	return Topic{
		ID:            t.TopicID,
		GroupID:       t.Group.GroupID,
		GroupName:     t.Group.Name,
		Type:          t.Type,
		AuthorName:    t.Owner.Name,
		AuthorID:      t.Owner.UserID,
		Text:          t.Talk.Text,
		LikesCount:    t.LikesCount,
		CommentsCount: t.CommentsCount,
		CreatedAt:     t.CreateTime,
		URL:           WebBase + "/topic/" + t.TopicID,
	}
}

func hashtagFromRaw(h rawHashtag) Hashtag {
	return Hashtag{
		ID:         h.HashtagID,
		Name:       h.Name,
		TopicCount: h.TopicCount,
		URL:        WebBase + "/hashtag/" + h.Name,
	}
}

// categoryIDs maps friendly slugs to Zsxq category_id values.
var categoryIDs = map[string]string{
	"all":     "0",
	"tech":    "1",
	"finance": "2",
	"design":  "3",
	"health":  "4",
	"career":  "5",
	"culture": "6",
	"hobby":   "7",
	"other":   "8",
}

// categoryID returns the numeric category ID for a slug, defaulting to "0" (all).
func categoryID(slug string) string {
	if slug == "" {
		return "0"
	}
	slug = strings.ToLower(strings.TrimSpace(slug))
	if id, ok := categoryIDs[slug]; ok {
		return id
	}
	// If the slug looks like a number, pass it through.
	if strings.IndexFunc(slug, func(r rune) bool { return r < '0' || r > '9' }) == -1 {
		return slug
	}
	return "0"
}
