package rssdetector

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestYouTubeChannelPath(t *testing.T) {
	h := youtubeHandler{}
	u, _ := url.Parse("https://www.youtube.com/channel/UCuAXFkgsw1L7xaCfnd5JJOw")
	if !h.Match(u) {
		t.Fatal("should match")
	}
	c := New(WithPlatformHandlers(true), WithRetry(RetryConfig{MaxAttempts: 1}))
	links, err := h.Discover(context.Background(), c, u)
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 {
		t.Fatalf("%+v", links)
	}
	want := "https://www.youtube.com/feeds/videos.xml?channel_id=UCuAXFkgsw1L7xaCfnd5JJOw"
	if links[0].URL != want {
		t.Fatalf("got %s", links[0].URL)
	}
	if links[0].Type != FeedTypeAtom {
		t.Fatalf("type %s", links[0].Type)
	}
}

func TestYouTubePlaylist(t *testing.T) {
	h := youtubeHandler{}
	u, _ := url.Parse("https://www.youtube.com/playlist?list=PLtest1234567890abcdef")
	c := New(WithRetry(RetryConfig{MaxAttempts: 1}))
	links, err := h.Discover(context.Background(), c, u)
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 || !strings.Contains(links[0].URL, "playlist_id=PLtest1234567890abcdef") {
		t.Fatalf("%+v", links)
	}
}

func TestYouTubeAlreadyFeed(t *testing.T) {
	h := youtubeHandler{}
	u, _ := url.Parse("https://www.youtube.com/feeds/videos.xml?channel_id=UCuAXFkgsw1L7xaCfnd5JJOw")
	c := New(WithRetry(RetryConfig{MaxAttempts: 1}))
	links, err := h.Discover(context.Background(), c, u)
	if err != nil || len(links) != 1 {
		t.Fatalf("%v %+v", err, links)
	}
}

func TestYouTubeHandleFromHTML(t *testing.T) {
	body, _ := os.ReadFile(filepath.Join("testdata", "youtube_channel.html"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write(body)
	}))
	defer srv.Close()

	// Point handler at our fixture server by temporarily using youtube host via Discover with rewritten URL
	// Discover fetches u.String() — so we need the request URL to be the test server.
	// Use a custom flow: extract ID unit test + Detect integration with httptest host is non-youtube.
	// Unit-test extraction:
	id := extractYouTubeChannelID(string(body))
	if id != "UCuAXFkgsw1L7xaCfnd5JJOw" {
		t.Fatalf("id = %q", id)
	}

	// Full handler with httptest: Match requires youtube host, so call Discover with youtube URL
	// but Client fetch will hit real network — instead inject transport rewrite.
	c := New(
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			req.URL, _ = url.Parse(srv.URL)
			req.Host = ""
			return http.DefaultTransport.RoundTrip(req)
		})}),
		WithRetry(RetryConfig{MaxAttempts: 1}),
		WithConfirmFeedLinks(false),
	)
	h := youtubeHandler{}
	u, _ := url.Parse("https://www.youtube.com/@testchannel")
	links, err := h.Discover(context.Background(), c, u)
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 {
		t.Fatalf("%+v", links)
	}
	if !strings.Contains(links[0].URL, "channel_id=UCuAXFkgsw1L7xaCfnd5JJOw") {
		t.Fatalf("%s", links[0].URL)
	}
}

func TestYouTubeNoInvent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<!DOCTYPE html><html><body>no channel id</body></html>"))
	}))
	defer srv.Close()
	c := New(
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			req2 := req.Clone(req.Context())
			req2.URL, _ = url.Parse(srv.URL)
			req2.RequestURI = ""
			return http.DefaultTransport.RoundTrip(req2)
		})}),
		WithRetry(RetryConfig{MaxAttempts: 1}),
	)
	h := youtubeHandler{}
	u, _ := url.Parse("https://www.youtube.com/@missing")
	links, err := h.Discover(context.Background(), c, u)
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 0 {
		t.Fatalf("should not invent: %+v", links)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
