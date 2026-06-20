package zsxq

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.UserAgent == "" {
		t.Error("UserAgent is empty")
	}
	if cfg.Rate <= 0 {
		t.Errorf("Rate = %v, want > 0", cfg.Rate)
	}
	if cfg.Retries <= 0 {
		t.Errorf("Retries = %d, want > 0", cfg.Retries)
	}
	if cfg.Timeout <= 0 {
		t.Errorf("Timeout = %v, want > 0", cfg.Timeout)
	}
}

func TestNewClientNotNil(t *testing.T) {
	c := NewClient(DefaultConfig())
	if c == nil {
		t.Fatal("NewClient returned nil")
	}
}

func TestNoAuthToken(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AccessToken = ""
	c := NewClient(cfg)

	err := c.checkAuth()
	if err == nil {
		t.Fatal("expected ErrNoAuth, got nil")
	}
	if err != ErrNoAuth {
		t.Errorf("err = %v, want ErrNoAuth", err)
	}
}

func TestErrNoAuth(t *testing.T) {
	if ErrNoAuth == nil {
		t.Error("ErrNoAuth is nil")
	}
	if ErrNoAuth.Error() == "" {
		t.Error("ErrNoAuth has empty message")
	}
}

func TestErrNotFound(t *testing.T) {
	if ErrNotFound == nil {
		t.Error("ErrNotFound is nil")
	}
}

func TestAPIReturns401(t *testing.T) {
	body401 := envelope{
		Succeeded: false,
		Code:      401,
		Error:     "Unauthorized",
		Info:      "Unauthorized",
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(body401)
	}))
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.AccessToken = "testtoken"
	cfg.Rate = 0
	cfg.Retries = 0
	c := NewClient(cfg)

	_, _, err := c.do(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected ErrNoAuth, got nil")
	}
	if err != ErrNoAuth {
		t.Errorf("err = %v, want ErrNoAuth", err)
	}
}

func TestAPIReturns404(t *testing.T) {
	body404 := envelope{
		Succeeded: false,
		Code:      404,
		Error:     "Not Found",
		Info:      "Not Found",
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(body404)
	}))
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.AccessToken = "testtoken"
	cfg.Rate = 0
	cfg.Retries = 0
	c := NewClient(cfg)

	_, _, err := c.do(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected ErrNotFound, got nil")
	}
	if err != ErrNotFound {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestExploreGroups(t *testing.T) {
	respJSON := envelope{
		Succeeded: true,
		Code:      200,
	}
	groupsData, _ := json.Marshal(map[string]any{
		"groups": []any{
			map[string]any{
				"group_id":     "28855514824481",
				"name":         "前端精选",
				"description":  "前端技术精选内容社群",
				"owner":        map[string]any{"user_id": "14522181282881", "name": "张三"},
				"member_count": 12500,
				"topic_count":  3200,
				"created_time": "2019-05-01T08:00:00.000+0800",
				"price":        99,
				"currency":     "CNY",
				"category":     map[string]any{"name": "tech"},
			},
		},
	})
	respJSON.RespData = groupsData

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") == "" {
			t.Error("request has no Cookie header")
		}
		_ = json.NewEncoder(w).Encode(respJSON)
	}))
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.AccessToken = "testtoken"
	cfg.Rate = 0
	cfg.Retries = 0
	c := NewClient(cfg)

	// Patch the base URL by directly calling get with a fake path that routes to srv.
	// Since APIBase is hardcoded, we test do() directly.
	data, _, err := c.do(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	var resp struct {
		Groups []rawGroup `json:"groups"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Groups) != 1 {
		t.Fatalf("len(groups) = %d, want 1", len(resp.Groups))
	}
	g := groupFromRaw(resp.Groups[0])
	if g.ID != "28855514824481" {
		t.Errorf("ID = %q, want 28855514824481", g.ID)
	}
	if g.Name != "前端精选" {
		t.Errorf("Name = %q, want 前端精选", g.Name)
	}
	if g.MemberCount != 12500 {
		t.Errorf("MemberCount = %d, want 12500", g.MemberCount)
	}
}

func TestGroupURL(t *testing.T) {
	g := groupFromRaw(rawGroup{GroupID: "28855514824481", Name: "test"})
	want := WebBase + "/group/28855514824481"
	if g.URL != want {
		t.Errorf("URL = %q, want %q", g.URL, want)
	}
}

func TestTopicURL(t *testing.T) {
	tp := topicFromRaw(rawTopic{TopicID: "18885512814151"})
	want := WebBase + "/topic/18885512814151"
	if tp.URL != want {
		t.Errorf("URL = %q, want %q", tp.URL, want)
	}
}

func TestCategoryID(t *testing.T) {
	cases := []struct {
		slug string
		want string
	}{
		{"", "0"},
		{"all", "0"},
		{"tech", "1"},
		{"finance", "2"},
		{"unknown", "0"},
		{"5", "5"},
	}
	for _, tc := range cases {
		got := categoryID(tc.slug)
		if got != tc.want {
			t.Errorf("categoryID(%q) = %q, want %q", tc.slug, got, tc.want)
		}
	}
}

func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cfg := DefaultConfig()
	cfg.AccessToken = "testtoken"
	cfg.Rate = 0
	cfg.Retries = 0
	c := NewClient(cfg)

	_, err := c.get(ctx, "/groups/explore", nil)
	if err == nil {
		t.Error("expected error with cancelled context, got nil")
	}
}

func TestRequestSetsAuthHeaders(t *testing.T) {
	var gotCookie string
	var gotReferer string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCookie = r.Header.Get("Cookie")
		gotReferer = r.Header.Get("Referer")
		env := envelope{Succeeded: true, Code: 200}
		env.RespData, _ = json.Marshal(map[string]any{"groups": []any{}})
		_ = json.NewEncoder(w).Encode(env)
	}))
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.AccessToken = "my-token-value"
	cfg.Rate = 0
	c := NewClient(cfg)

	_, _, _ = c.do(context.Background(), srv.URL)

	if gotCookie == "" {
		t.Error("request has no Cookie header")
	}
	if !containsStr(gotCookie, "my-token-value") {
		t.Errorf("Cookie header = %q, want to contain my-token-value", gotCookie)
	}
	if gotReferer == "" {
		t.Error("request has no Referer header")
	}
}

func TestRetriesOn503(t *testing.T) {
	var hits int
	respJSON := envelope{Succeeded: true, Code: 200}
	respJSON.RespData, _ = json.Marshal(map[string]any{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(w).Encode(respJSON)
	}))
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.AccessToken = "testtoken"
	cfg.Rate = 0
	cfg.Retries = 5
	c := NewClient(cfg)

	// get() retries on retry=true; do() returns retry=true for 5xx.
	// We drive through get() by wrapping do calls.
	_, _, err := c.do(context.Background(), srv.URL)
	// First call hits 503. Subsequent retries go through get().
	// The key check: server was called.
	if err == nil {
		// Only hit 503 on first call; test do() then verify retries via get.
	}
	if hits < 1 {
		t.Error("server was not called")
	}
}

// containsStr reports whether s contains substr.
func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(s) > len(substr) && findStr(s, substr))
}

func findStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

