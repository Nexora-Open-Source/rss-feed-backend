/*
Package utils provides utility functions for RSS feed parsing.

Key Functions:
  - FetchRSSFeed: Parses an RSS feed from a URL and returns a slice of feed items.

Dependencies:
  - Uses the `gofeed` library for RSS parsing.

Usage:

	items, err := FetchRSSFeed("https://example.com/rss")
	if err != nil {
	    log.Fatalf("Failed to fetch RSS feed: %v", err)
	}
*/
package utils

import (
	"time"

	"github.com/mmcdole/gofeed"
)

// FeedItem represents an RSS feed item
type FeedItem struct {
	Title       string `datastore:"title,noindex"` // noindex to exclude from indexes
	Link        string `datastore:"link"`
	Description string `datastore:"description,noindex"`
	Author      string `datastore:"author,noindex"`
	PubDate     string `datastore:"pub_date,noindex"`
}

/*
FetchRSSFeed fetches and parses an RSS feed from the given URL.

Parameters:
  - url: The URL of the RSS feed.

Returns:
  - A slice of FeedItem objects containing parsed RSS feed data.
  - An error if parsing fails.

Example:

	items, err := FetchRSSFeed("https://example.com/rss")
	if err != nil {
	    log.Fatalf("Failed to fetch RSS feed: %v", err)
	}

FeedItem Structure:
  - Title:       The title of the RSS feed item.
  - Link:        The URL link to the original article.
  - Description: A short description of the RSS feed item.
  - PubDate:     The publication date of the RSS feed item.
*/
func FetchRSSFeed(url string) ([]*FeedItem, error) {
	parser := gofeed.NewParser()
	feed, err := parser.ParseURL(url)
	if err != nil {
		return nil, err
	}

	var items []*FeedItem
	for _, entry := range feed.Items {
		pubDate, _ := time.Parse(time.RFC1123Z, entry.Published)
		items = append(items, &FeedItem{
			Title:       entry.Title,
			Link:        entry.Link,
			Description: entry.Description,
			Author:      handleAuthor(entry),
			PubDate:     pubDate.Format(time.RFC3339),
		})
	}
	return items, nil
}

func handleAuthor(entry *gofeed.Item) string {
	if entry.Author != nil {
		return entry.Author.Name
	}
	return "Unknown"
}
