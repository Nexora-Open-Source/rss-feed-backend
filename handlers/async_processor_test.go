package handlers

import (
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAsyncProcessor(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	processor := NewAsyncProcessor(2, 10, true, 0.8, 5*time.Second, logger, nil, nil)
	defer processor.Stop()

	assert.NotNil(t, processor)
	assert.Equal(t, logger, processor.logger)
}

func TestAsyncProcessorSubmitJob(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	processor := NewAsyncProcessor(1, 5, true, 0.8, 5*time.Second, logger, nil, nil)
	defer processor.Stop()

	// Submit a job
	jobID, err := processor.SubmitJob("https://example.com/rss.xml", "test-request-123")

	require.NoError(t, err)
	assert.NotEmpty(t, jobID)
	assert.True(t, len(jobID) > 10) // Should be a reasonable length
}

func TestAsyncProcessorGetJobStatus(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	processor := NewAsyncProcessor(1, 5, true, 0.8, 5*time.Second, logger, nil, nil)
	defer processor.Stop()

	// Submit a job
	jobID, err := processor.SubmitJob("https://example.com/rss.xml", "test-request-123")
	require.NoError(t, err)

	// Check initial status
	status, exists := processor.GetJobStatus(jobID)
	assert.True(t, exists)
	assert.Equal(t, "pending", status.Status)
	assert.Equal(t, jobID, status.JobID)
	assert.Equal(t, "https://example.com/rss.xml", status.URL)
}

func TestAsyncProcessorGetJobStatusNotFound(t *testing.T) {
	logger := logrus.New()

	processor := NewAsyncProcessor(1, 5, true, 0.8, 5*time.Second, logger, nil, nil)
	defer processor.Stop()

	// Check status of non-existent job
	status, exists := processor.GetJobStatus("non-existent-job")
	assert.False(t, exists)
	assert.Nil(t, status)
}

func TestAsyncProcessorStop(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	processor := NewAsyncProcessor(1, 5, true, 0.8, 5*time.Second, logger, nil, nil)

	// Submit a job to ensure processor is running
	jobID, err := processor.SubmitJob("https://example.com/rss.xml", "test-request-123")
	require.NoError(t, err)

	// Stop the processor
	assert.NotPanics(t, func() {
		processor.Stop()
	})

	// Verify job status is still accessible
	status, exists := processor.GetJobStatus(jobID)
	assert.True(t, exists)
	assert.NotNil(t, status)
}

// Benchmark tests
func BenchmarkAsyncProcessorSubmitJob(b *testing.B) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	processor := NewAsyncProcessor(2, 100, true, 0.8, 5*time.Second, logger, nil, nil)
	defer processor.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		processor.SubmitJob("https://example.com/rss.xml", "test-request")
	}
}
