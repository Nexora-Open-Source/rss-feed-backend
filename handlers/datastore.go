package handlers

import (
	"context"

	"github.com/Nexora-Open-Source/rss-feed-backend/utils"

	"cloud.google.com/go/datastore"
)

// SaveToDatastore saves RSS feed items to Google CLoud Datastore
func SaveToDatastore(items []*utils.FeedItem) error {
	ctx := context.Background()
	for _, item := range items {
		// Use the Link as the unique key to prevent duplicates
		key := datastore.NameKey("FeedItem", item.Link, nil)
		_, err := datastoreClient.Put(ctx, key, item)
		if err != nil {
			return err
		}
	}
	return nil
}

// FetchFeedItems retrieves all feed items from Datastore
func FetchFeedItems() ([]*utils.FeedItem, error) {
	ctx := context.Background()
	query := datastore.NewQuery("FeedItem")
	var items []*utils.FeedItem
	_, err := datastoreClient.GetAll(ctx, query, &items)
	if err != nil {
		return nil, err
	}
	return items, nil
}
