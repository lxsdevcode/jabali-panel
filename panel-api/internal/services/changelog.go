package services

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// M53 Updates Center — changelog feed (ADR-0118).
//
// Pulls release notes from the public GitHub mirror's releases API. The URL is
// a fixed constant (no user input) so there is no SSRF surface. Results are
// cached for ttl; on any fetch error the last good cache is served, so a
// GitHub blip or offline box degrades to "stale" rather than empty. Until the
// first release is cut the API returns [] and the UI shows an empty state.

const changelogReleasesURL = "https://api.github.com/repos/shukiv/jabali-panel/releases?per_page=20"

// ChangelogEntry is one published release, trimmed to what the UI renders.
type ChangelogEntry struct {
	Tag         string `json:"tag"`
	Name        string `json:"name"`
	PublishedAt string `json:"published_at"`
	Body        string `json:"body"`
}

// githubRelease is the subset of the GitHub releases payload we read.
type githubRelease struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	PublishedAt string `json:"published_at"`
	Body        string `json:"body"`
	Draft       bool   `json:"draft"`
	Prerelease  bool   `json:"prerelease"`
}

// ChangelogService fetches + caches the release feed.
type ChangelogService struct {
	url    string
	ttl    time.Duration
	client *http.Client

	mu       sync.Mutex
	cached   []ChangelogEntry
	cachedAt time.Time
	haveData bool
}

// NewChangelogService returns a service with a 6h cache + 10s fetch timeout.
func NewChangelogService() *ChangelogService {
	return &ChangelogService{
		url:    changelogReleasesURL,
		ttl:    6 * time.Hour,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Get returns the cached entries (refreshing past ttl) plus the cache
// timestamp. A fetch error with a warm cache returns the stale data and no
// error; a fetch error with a cold cache returns the error.
func (s *ChangelogService) Get(ctx context.Context) ([]ChangelogEntry, time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.haveData && time.Since(s.cachedAt) < s.ttl {
		return s.cached, s.cachedAt, nil
	}

	entries, err := s.fetch(ctx)
	if err != nil {
		if s.haveData {
			return s.cached, s.cachedAt, nil
		}
		return nil, time.Time{}, err
	}

	s.cached = entries
	s.cachedAt = time.Now()
	s.haveData = true
	return s.cached, s.cachedAt, nil
}

func (s *ChangelogService) fetch(ctx context.Context) ([]ChangelogEntry, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "jabali-panel")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, &httpStatusError{code: resp.StatusCode}
	}

	var releases []githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, err
	}

	entries := make([]ChangelogEntry, 0, len(releases))
	for _, r := range releases {
		if r.Draft {
			continue // unpublished
		}
		entries = append(entries, ChangelogEntry{
			Tag:         r.TagName,
			Name:        r.Name,
			PublishedAt: r.PublishedAt,
			Body:        r.Body,
		})
	}
	return entries, nil
}

type httpStatusError struct{ code int }

func (e *httpStatusError) Error() string {
	return "changelog: unexpected status " + http.StatusText(e.code)
}
