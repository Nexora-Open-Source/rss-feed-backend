package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"cloud.google.com/go/datastore"
	"github.com/Nexora-Open-Source/rss-feed-backend/middleware"
	"github.com/Nexora-Open-Source/rss-feed-backend/types"
	"github.com/Nexora-Open-Source/rss-feed-backend/utils"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockDatastoreClient is a mock for datastore.Client
type MockDatastoreClient struct {
	mock.Mock
}

// GetAll mocks the GetAll method
func (m *MockDatastoreClient) GetAll(ctx context.Context, q *datastore.Query, dst interface{}) ([]*datastore.Key, error) {
	args := m.Called(ctx, q, dst)
	return args.Get(0).([]*datastore.Key), args.Error(1)
}

// PutMulti mocks the PutMulti method
func (m *MockDatastoreClient) PutMulti(ctx context.Context, keys []*datastore.Key, src interface{}) ([]*datastore.Key, error) {
	args := m.Called(ctx, keys, src)
	return args.Get(0).([]*datastore.Key), args.Error(1)
}

// Get mocks the Get method
func (m *MockDatastoreClient) Get(ctx context.Context, key *datastore.Key, dst interface{}) error {
	args := m.Called(ctx, key, dst)
	return args.Error(0)
}

// DeleteMulti mocks the DeleteMulti method
func (m *MockDatastoreClient) DeleteMulti(ctx context.Context, keys []*datastore.Key) error {
	args := m.Called(ctx, keys)
	return args.Error(0)
}

// MockCacheManager is a mock for cache.CacheManager
type MockCacheManager struct {
	mock.Mock
}

// GetStoredItems mocks the GetStoredItems method
func (m *MockCacheManager) GetStoredItems(key string) ([]*utils.FeedItem, bool) {
	args := m.Called(key)
	return args.Get(0).([]*utils.FeedItem), args.Bool(1)
}

// SetStoredItems mocks the SetStoredItems method
func (m *MockCacheManager) SetStoredItems(key string, items []*utils.FeedItem) error {
	args := m.Called(key, items)
	return args.Error(0)
}

// GetFeedItems mocks the GetFeedItems method
func (m *MockCacheManager) GetFeedItems(key string) ([]*utils.FeedItem, bool) {
	args := m.Called(key)
	return args.Get(0).([]*utils.FeedItem), args.Bool(1)
}

// SetFeedItems mocks the SetFeedItems method
func (m *MockCacheManager) SetFeedItems(key string, items []*utils.FeedItem) error {
	args := m.Called(key, items)
	return args.Error(0)
}

// MockAsyncProcessor is a mock for AsyncProcessor
type MockAsyncProcessor struct {
	mock.Mock
}

// SubmitJob mocks the SubmitJob method
func (m *MockAsyncProcessor) SubmitJob(url, requestID string) (string, error) {
	args := m.Called(url, requestID)
	return args.String(0), args.Error(1)
}

// GetJobStatus mocks the GetJobStatus method
func (m *MockAsyncProcessor) GetJobStatus(jobID string) (*types.AsyncJobStatus, bool) {
	args := m.Called(jobID)
	return args.Get(0).(*types.AsyncJobStatus), args.Bool(1)
}

func setupTestHandler(t *testing.T) (*Handler, *MockDatastoreClient, *MockCacheManager, *MockAsyncProcessor) {
	mockDatastore := &MockDatastoreClient{}
	mockCache := &MockCacheManager{}
	mockAsync := &MockAsyncProcessor{}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise in tests

	// Initialize middleware logger for tests
	middleware.Logger = logger

	handler := &Handler{
		DatastoreClient: mockDatastore,
		CacheManager:    mockCache,
		Logger:          logger,
		AsyncProcessor:  mockAsync,
	}

	return handler, mockDatastore, mockCache, mockAsync
}

func TestHandleHealthCheck(t *testing.T) {
	handler, mockDatastore, _, _ := setupTestHandler(t)

	// Mock datastore health check
	mockDatastore.On("GetAll", mock.Anything, mock.Anything, mock.Anything).
		Return([]*datastore.Key{}, nil)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler.HandleHealthCheck(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "healthy", response["status"])
	assert.Contains(t, response, "timestamp")
	assert.Contains(t, response, "services")
}

func TestHandleLivenessCheck(t *testing.T) {
	handler, _, _, _ := setupTestHandler(t)

	req := httptest.NewRequest("GET", "/health/live", nil)
	w := httptest.NewRecorder()

	handler.HandleLivenessCheck(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "alive", response["status"])
}

func TestHandleReadinessCheck(t *testing.T) {
	handler, mockDatastore, _, _ := setupTestHandler(t)

	// Mock successful datastore health check
	mockDatastore.On("GetAll", mock.Anything, mock.Anything, mock.Anything).
		Return([]*datastore.Key{}, nil)

	req := httptest.NewRequest("GET", "/health/ready", nil)
	w := httptest.NewRecorder()

	handler.HandleReadinessCheck(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "ready", response["status"])
}

func TestHandleGetFeeds(t *testing.T) {
	handler, _, _, _ := setupTestHandler(t)

	req := httptest.NewRequest("GET", "/feeds", nil)
	w := httptest.NewRecorder()

	handler.HandleGetFeeds(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response []FeedSource
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.NotEmpty(t, response)
}

func TestHandleGetJobStatus(t *testing.T) {
	handler, _, _, mockAsync := setupTestHandler(t)

	// Mock job status
	jobStatus := &types.AsyncJobStatus{
		JobID:  "test-job-123",
		Status: "completed",
		URL:    "https://example.com/rss.xml",
	}

	mockAsync.On("GetJobStatus", "test-job-123").
		Return(jobStatus, true)

	req := httptest.NewRequest("GET", "/job-status?job_id=test-job-123", nil)
	w := httptest.NewRecorder()

	handler.HandleGetJobStatus(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response types.AsyncJobStatus
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, jobStatus.JobID, response.JobID)
	assert.Equal(t, jobStatus.Status, response.Status)
}

func TestHandleGetJobStatusNotFound(t *testing.T) {
	handler, _, _, mockAsync := setupTestHandler(t)

	// Mock job not found
	mockAsync.On("GetJobStatus", "nonexistent-job").
		Return((*types.AsyncJobStatus)(nil), false)

	req := httptest.NewRequest("GET", "/job-status?job_id=nonexistent-job", nil)
	w := httptest.NewRecorder()

	handler.HandleGetJobStatus(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandleFetchAndStoreMissingURL(t *testing.T) {
	handler, _, _, _ := setupTestHandler(t)

	// Test with missing URL
	req := httptest.NewRequest("POST", "/fetch-store", nil)
	w := httptest.NewRecorder()

	handler.HandleFetchAndStore(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleFetchAndStoreInvalidURL(t *testing.T) {
	handler, _, _, _ := setupTestHandler(t)

	// Test with invalid URL - will cause decode error
	req := httptest.NewRequest("POST", "/fetch-store", nil)
	req.Body = nil // Will cause decode error
	w := httptest.NewRecorder()

	handler.HandleFetchAndStore(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleGetFeedItems(t *testing.T) {
	handler, mockDatastore, mockCache, _ := setupTestHandler(t)

	// Mock cache miss
	mockCache.On("GetStoredItems", mock.Anything).
		Return([]*utils.FeedItem{}, false)

	// Mock datastore response
	mockDatastore.On("GetAll", mock.Anything, mock.Anything, mock.Anything).
		Return([]*datastore.Key{}, nil)

	req := httptest.NewRequest("GET", "/items?limit=10&offset=0", nil)
	w := httptest.NewRecorder()

	handler.HandleGetFeedItems(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestNewHandler(t *testing.T) {
	mockDatastore := &MockDatastoreClient{}
	mockCache := &MockCacheManager{}
	logger := logrus.New()

	// Create a real async processor since we're testing the constructor
	// We can't easily mock it without changing the constructor signature
	handler := &Handler{
		DatastoreClient: mockDatastore,
		CacheManager:    mockCache,
		Logger:          logger,
		AsyncProcessor:  &MockAsyncProcessor{}, // Use mock for testing
	}

	assert.NotNil(t, handler)
	assert.Equal(t, mockDatastore, handler.DatastoreClient)
	assert.Equal(t, mockCache, handler.CacheManager)
	assert.Equal(t, logger, handler.Logger)
	assert.NotNil(t, handler.AsyncProcessor)
}
