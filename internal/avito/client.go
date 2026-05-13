package avito

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Client struct {
	userID string

	cacheTTL   time.Duration
	httpClient *http.Client

	cacheMu        sync.RWMutex
	cachedResponse *ReviewsResponse
	cachedAt       time.Time
}

type ReviewsResponse struct {
	Reviews   []ReviewItem `json:"reviews"`
	FetchedAt string       `json:"fetchedAt"`
	SourceURL string       `json:"sourceUrl"`
}

type ReviewItem struct {
	ID         string  `json:"id"`
	Author     string  `json:"author"`
	Role       string  `json:"role"`
	DateLabel  string  `json:"dateLabel,omitempty"`
	Rating     float64 `json:"rating"`
	Text       string  `json:"text,omitempty"`
	ItemTitle  string  `json:"itemTitle,omitempty"`
	StageTitle string  `json:"stageTitle,omitempty"`
	AvatarURL  string  `json:"avatarUrl,omitempty"`
}

func NewClient(userID string, cacheTTL time.Duration, httpClient *http.Client) *Client {
	if httpClient == nil {
		panic("httpClient is nil")
	}

	return &Client{
		userID: userID,

		cacheTTL:   cacheTTL,
		httpClient: httpClient,
	}
}

func (c *Client) apiURL() string {
	return fmt.Sprintf("https://avito.ru/web/6/user/%s/ratings", c.userID)
}

func (c *Client) sourceURL() string {
	return fmt.Sprintf("https://avito.ru/user/%s/profile?src=ratings", c.userID)
}

func (c *Client) FetchReviews(ctx context.Context, log *slog.Logger) (ReviewsResponse, error) {
	if cached, ok := c.loadFromCache(); ok {
		return cached, nil
	}

	payload, err := c.pull(ctx, log)
	if err != nil {
		return ReviewsResponse{}, err
	}

	normalized := normalizePayload(payload, c.sourceURL())
	c.store(normalized)

	return normalized, nil
}

func (c *Client) pull(ctx context.Context, log *slog.Logger) (avitoPayload, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiURL(), nil)
	if err != nil {
		return avitoPayload{}, fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", nextUserAgent(ctx, log))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return avitoPayload{}, fmt.Errorf("do request: %w", err)
	}
	defer func() {
		if err = resp.Body.Close(); err != nil {
			log.ErrorContext(ctx, "failed to close response body", "error", err)
		}
	}()

	if resp.StatusCode >= http.StatusBadRequest {
		return avitoPayload{}, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var payload avitoPayload
	if err = json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return avitoPayload{}, fmt.Errorf("decode response: %w", err)
	}

	if len(payload.Entries) == 0 {
		return avitoPayload{}, errors.New("response does not contain entries")
	}

	return payload, nil
}

func (c *Client) loadFromCache() (ReviewsResponse, bool) {
	c.cacheMu.RLock()
	defer c.cacheMu.RUnlock()

	if c.cachedResponse == nil {
		return ReviewsResponse{}, false
	}

	if time.Since(c.cachedAt) > c.cacheTTL {
		return ReviewsResponse{}, false
	}

	return *c.cachedResponse, true
}

func (c *Client) store(resp ReviewsResponse) {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()

	copied := resp
	c.cachedResponse = &copied
	c.cachedAt = time.Now()
}

type avitoPayload struct {
	Entries []avitoEntry `json:"entries"`
}

type avitoEntry struct {
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value"`
}

type ratingValue struct {
	ID           json.RawMessage   `json:"id"`
	Title        string            `json:"title"`
	TitleCaption string            `json:"titleCaption"`
	Rated        string            `json:"rated"`
	Score        *float64          `json:"score"`
	StageTitle   string            `json:"stageTitle"`
	ItemTitle    string            `json:"itemTitle"`
	TextSections []textSection     `json:"textSections"`
	Avatar       map[string]string `json:"avatar"`
	Answer       *ratingAnswer     `json:"answer"`
}

type textSection struct {
	Text string `json:"text"`
}

type ratingAnswer struct {
	AnswerID json.RawMessage `json:"answerId"`
}

func normalizePayload(payload avitoPayload, sourceURL string) ReviewsResponse {
	reviews := make([]ReviewItem, 0, len(payload.Entries))

	for _, entry := range payload.Entries {
		if entry.Type != "rating" || len(entry.Value) == 0 {
			continue
		}

		var value ratingValue
		if err := json.Unmarshal(entry.Value, &value); err != nil {
			continue
		}

		if review := mapReview(value); review != nil {
			reviews = append(reviews, *review)
		}
	}

	return ReviewsResponse{
		Reviews:   reviews,
		FetchedAt: time.Now().UTC().Format(time.RFC3339),
		SourceURL: sourceURL,
	}
}

func mapReview(value ratingValue) *ReviewItem {
	id := firstNonEmpty(rawToString(value.ID), rawToString(answerID(value.Answer)))
	if id == "" {
		return nil
	}

	rating := 0.0
	if value.Score != nil {
		rating = *value.Score
	}

	return &ReviewItem{
		ID:         id,
		Author:     fallbackString(value.Title, "Аноним"),
		Role:       fallbackString(value.TitleCaption, "Покупатель"),
		DateLabel:  strings.TrimSpace(value.Rated),
		Rating:     rating,
		Text:       collectText(value.TextSections),
		ItemTitle:  strings.TrimSpace(value.ItemTitle),
		StageTitle: strings.TrimSpace(value.StageTitle),
		AvatarURL:  pickAvatar(value.Avatar),
	}
}

func collectText(sections []textSection) string {
	var builder strings.Builder

	for _, section := range sections {
		chunk := strings.TrimSpace(section.Text)
		if chunk == "" {
			continue
		}

		if builder.Len() > 0 {
			builder.WriteString("\n\n")
		}
		builder.WriteString(chunk)
	}

	return builder.String()
}

func pickAvatar(avatars map[string]string) string {
	if len(avatars) == 0 {
		return ""
	}

	preferred := []string{"128x128", "100x100", "64x64", "50x50"}
	for _, key := range preferred {
		if url := strings.TrimSpace(avatars[key]); url != "" {
			return url
		}
	}

	for _, url := range avatars {
		if trimmed := strings.TrimSpace(url); trimmed != "" {
			return trimmed
		}
	}

	return ""
}

func answerID(answer *ratingAnswer) json.RawMessage {
	if answer == nil {
		return nil
	}

	return answer.AnswerID
}

func rawToString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var str string
	if err := json.Unmarshal(raw, &str); err == nil && str != "" {
		return str
	}

	var num json.Number
	if err := json.Unmarshal(raw, &num); err == nil {
		return num.String()
	}

	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}

	return ""
}

func fallbackString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}

	return value
}
