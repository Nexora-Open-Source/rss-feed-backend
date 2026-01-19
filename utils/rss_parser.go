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
	"crypto/md5"
	"fmt"
	"net/url"
	"strings"
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

// Validate validates the FeedItem fields
func (f *FeedItem) Validate() error {
	var errors []string

	// Validate Title
	if strings.TrimSpace(f.Title) == "" {
		errors = append(errors, "title cannot be empty")
	} else if len(f.Title) > 500 {
		errors = append(errors, "title cannot exceed 500 characters")
	}

	// Validate Link
	if strings.TrimSpace(f.Link) == "" {
		errors = append(errors, "link cannot be empty")
	} else if _, err := url.ParseRequestURI(f.Link); err != nil {
		errors = append(errors, "link must be a valid URL")
	}

	// Validate Description
	if len(f.Description) > 2000 {
		errors = append(errors, "description cannot exceed 2000 characters")
	}

	// Validate Author
	if len(f.Author) > 100 {
		errors = append(errors, "author cannot exceed 100 characters")
	}

	// Validate PubDate
	if strings.TrimSpace(f.PubDate) != "" {
		if _, err := time.Parse(time.RFC3339, f.PubDate); err != nil {
			errors = append(errors, "pub_date must be in RFC3339 format")
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("validation failed: %s", strings.Join(errors, ", "))
	}

	return nil
}

// GenerateContentHash generates a hash for content-based duplicate detection
func (f *FeedItem) GenerateContentHash() string {
	content := f.Title + f.Description + f.Author
	return fmt.Sprintf("%x", md5.Sum([]byte(content)))
}

// IsDuplicate checks if this item is likely a duplicate of another
func (f *FeedItem) IsDuplicate(other *FeedItem) bool {
	// Exact link match
	if f.Link == other.Link {
		return true
	}

	// Content similarity check
	if f.Title == other.Title && f.Author == other.Author {
		return true
	}

	// Hash-based duplicate detection
	return f.GenerateContentHash() == other.GenerateContentHash()
}

// Sanitize sanitizes the FeedItem fields
func (f *FeedItem) Sanitize() {
	f.Title = strings.TrimSpace(f.Title)
	f.Link = strings.TrimSpace(f.Link)
	f.Description = strings.TrimSpace(f.Description)
	f.Author = strings.TrimSpace(f.Author)
	f.PubDate = strings.TrimSpace(f.PubDate)
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
		item := &FeedItem{
			Title:       entry.Title,
			Link:        entry.Link,
			Description: entry.Description,
			Author:      handleAuthor(entry),
			PubDate:     pubDate.Format(time.RFC3339),
		}

		// Sanitize the item
		item.Sanitize()

		// Validate the item
		if err := item.Validate(); err != nil {
			// Log validation error but continue processing other items
			continue
		}

		items = append(items, item)
	}
	return items, nil
}

func handleAuthor(entry *gofeed.Item) string {
	if entry.Author != nil {
		return entry.Author.Name
	}
	return "Unknown"
}
