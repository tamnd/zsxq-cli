package zsxq

// Group is a Zsxq knowledge group (知识圈).
type Group struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	OwnerName   string  `json:"owner_name"`
	OwnerID     string  `json:"owner_id"`
	MemberCount int     `json:"member_count"`
	TopicCount  int     `json:"topic_count"`
	CreatedAt   string  `json:"created_at,omitempty"`
	Price       float64 `json:"price"`
	Currency    string  `json:"currency"`
	Category    string  `json:"category,omitempty"`
	URL         string  `json:"url"`
}

// Topic is a post or discussion in a Zsxq group.
type Topic struct {
	ID            string `json:"id"`
	GroupID       string `json:"group_id"`
	GroupName     string `json:"group_name"`
	Type          string `json:"type"`
	AuthorName    string `json:"author_name"`
	AuthorID      string `json:"author_id"`
	Text          string `json:"text,omitempty"`
	LikesCount    int    `json:"likes_count"`
	CommentsCount int    `json:"comments_count"`
	CreatedAt     string `json:"created_at,omitempty"`
	URL           string `json:"url"`
}

// SearchResult is one item from a Zsxq search response.
type SearchResult struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	MemberCount int    `json:"member_count,omitempty"`
	URL         string `json:"url"`
}

// Hashtag is a trending hashtag on Zsxq.
type Hashtag struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	TopicCount int    `json:"topic_count"`
	URL        string `json:"url"`
}
