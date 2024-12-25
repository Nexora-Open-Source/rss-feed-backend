/*
Package handlers provides functions to interact with Google Cloud Datastore.

Key Functions:
  - SaveToDatastore: Stores RSS feed items in Datastore.
  - FetchFeedItems: Retrieves stored feed items from Datastore.
*/
package handlers

import (
	"context"

	"github.com/Nexora-Open-Source/rss-feed-backend/utils"

	"cloud.google.com/go/datastore"
)

/*
SaveToDatastore saves a list of RSS feed items to Google Cloud Datastore.

Parameters:
  - items: A slice of FeedItem objects to store.

Errors:

	Returns an error if Datastore operation fails.

Usage:

	items := []*FeedItem{...}
	err := SaveToDatastore(items)
	if err != nil {
	    log.Fatalf("Failed to save feed items: %v", err)
	}
*/
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

/*
FetchFeedItems retrieves all RSS feed items stored in Google Cloud Datastore.

Returns:
  - A slice of FeedItem objects.
  - An error if Datastore operation fails.

Usage:

	items, err := FetchFeedItems()
	if err != nil {
	    log.Fatalf("Failed to fetch feed items: %v", err)
	}
*/
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
