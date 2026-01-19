package utils

import (
	"testing"

	"github.com/mmcdole/gofeed"
	"github.com/stretchr/testify/assert"
)

func TestGenerateRequestID(t *testing.T) {
	// Test that request IDs are generated
	id1 := GenerateRequestID()
	id2 := GenerateRequestID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)

	// Test that IDs are expected length (14 timestamp + 1 dash + 8 random = 23)
	assert.Equal(t, 23, len(id1))
	assert.Equal(t, 23, len(id2))
}

func TestRandomString(t *testing.T) {
	// Test different lengths
	for length := 1; length <= 20; length++ {
		result := RandomString(length)
		assert.Equal(t, length, len(result))

		// Test that it contains only valid base64 URL characters
		for _, char := range result {
			assert.Contains(t, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_", string(char))
		}
	}
}

func TestHandleAuthor(t *testing.T) {
	tests := []struct {
		name     string
		entry    *gofeed.Item
		expected string
	}{
		{
			name: "author with name",
			entry: &gofeed.Item{
				Author: &gofeed.Person{Name: "John Doe"},
			},
			expected: "John Doe",
		},
		{
			name: "nil author",
			entry: &gofeed.Item{
				Author: nil,
			},
			expected: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handleAuthor(tt.entry)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFetchRSSFeedValidURL(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Test with a real RSS feed URL
	url := "https://feeds.bbci.co.uk/news/rss.xml"

	items, err := FetchRSSFeed(url)

	if err != nil {
		t.Logf("Warning: Could not fetch RSS feed (may be network issue): %v", err)
		t.Skip("Network dependent test")
		return
	}

	assert.NotEmpty(t, items)

	// Verify structure of first item
	if len(items) > 0 {
		item := items[0]
		assert.NotEmpty(t, item.Title)
		assert.NotEmpty(t, item.Link)
		assert.NotEmpty(t, item.PubDate)
	}
}

func TestFetchRSSFeedInvalidURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"empty URL", ""},
		{"invalid format", "not-a-url"},
		{"non-existent domain", "https://thisdomaindoesnotexist12345.com/rss.xml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items, err := FetchRSSFeed(tt.url)

			assert.Error(t, err)
			assert.Nil(t, items)
		})
	}
}

// Benchmark tests
func BenchmarkGenerateRequestID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GenerateRequestID()
	}
}

func BenchmarkRandomString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		RandomString(10)
	}
}
