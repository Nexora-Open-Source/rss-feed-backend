package utils

import (
	"time"

	"github.com/mmcdole/gofeed"
)

// FeedItem represents an RSS feed item
type FeedItem struct {
	Title       string `datastore:"title"`
	Link        string `datastore:"link"`
	Description string `datastore:"description"`
	PubDate     string `datastore:"pub_date"`
}

// FetchRSSFeed fetches and parses an RSS feed from the given URL
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
			PubDate:     pubDate.Format(time.RFC3339),
		})
	}
	return items, nil
}
