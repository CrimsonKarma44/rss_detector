package rssdetector

import (
	"errors"
	"net/url"
	"testing"
)

func TestNormalizeInput(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr error
	}{
		{"https", "https://example.com/path", "https://example.com/path", nil},
		{"http", "http://example.com", "http://example.com", nil},
		{"bare host", "example.com/feed", "https://example.com/feed", nil},
		{"whitespace", "  https://example.com  ", "https://example.com", nil},
		{"strip fragment", "https://example.com/a#section", "https://example.com/a", nil},
		{"empty", "", "", ErrInvalidURL},
		{"spaces only", "   ", "", ErrInvalidURL},
		{"ftp", "ftp://example.com", "", ErrInvalidURL},
		{"no host", "https://", "", ErrInvalidURL},
		{"preserve query", "https://youtube.com/watch?v=abc", "https://youtube.com/watch?v=abc", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := NormalizeInput(tt.in)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if u.String() != tt.want {
				t.Fatalf("got %q want %q", u.String(), tt.want)
			}
		})
	}
}

func TestResolveURL(t *testing.T) {
	base, _ := url.Parse("https://example.com/blog/post")
	tests := []struct {
		name string
		ref  string
		want string
	}{
		{"absolute", "https://cdn.example.com/feed.xml", "https://cdn.example.com/feed.xml"},
		{"root relative", "/rss.xml", "https://example.com/rss.xml"},
		{"relative", "feed", "https://example.com/blog/feed"},
		{"query", "/?feed=rss2", "https://example.com/?feed=rss2"},
		{"strip fragment", "/feed#top", "https://example.com/feed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveURL(base, tt.ref)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
	if _, err := ResolveURL(nil, "/x"); !errors.Is(err, ErrInvalidURL) {
		t.Fatalf("nil base: %v", err)
	}
	if _, err := ResolveURL(base, ""); !errors.Is(err, ErrInvalidURL) {
		t.Fatalf("empty ref: %v", err)
	}
}

func TestCanonicalFeedURL(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"HTTPS://Example.COM:443/Feed", "https://example.com/Feed"},
		{"http://example.com:80/rss", "http://example.com/rss"},
		{"https://example.com:8443/a", "https://example.com:8443/a"},
		{"https://example.com/a#frag", "https://example.com/a"},
		{"https://EXAMPLE.com", "https://example.com/"},
	}
	for _, tt := range tests {
		got := CanonicalFeedURL(tt.in)
		if got != tt.want {
			t.Errorf("CanonicalFeedURL(%q) = %q want %q", tt.in, got, tt.want)
		}
	}
}

func TestOrigin(t *testing.T) {
	u, _ := url.Parse("https://blog.example.com/path?q=1")
	if got := Origin(u); got != "https://blog.example.com" {
		t.Fatalf("got %q", got)
	}
	if Origin(nil) != "" {
		t.Fatal("nil origin")
	}
}

func TestIsYouTubeHost(t *testing.T) {
	yes := []string{"youtube.com", "www.youtube.com", "m.youtube.com", "youtu.be", "music.youtube.com"}
	no := []string{"notyoutube.com", "example.com", "youtube.com.evil"}
	for _, h := range yes {
		if !IsYouTubeHost(h) {
			t.Errorf("expected youtube: %s", h)
		}
	}
	for _, h := range no {
		if IsYouTubeHost(h) {
			t.Errorf("unexpected youtube: %s", h)
		}
	}
}
