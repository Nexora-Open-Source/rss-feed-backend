/*
Package container provides dependency injection capabilities for the RSS Feed backend.

This package implements a simple dependency injection container that helps manage
service dependencies and reduces tight coupling between components.
*/
package container

import (
	"fmt"
	"sync"

	"cloud.google.com/go/datastore"
	"github.com/Nexora-Open-Source/rss-feed-backend/cache"
	"github.com/Nexora-Open-Source/rss-feed-backend/handlers"
	"github.com/sirupsen/logrus"
)

// Container holds all service dependencies
type Container struct {
	mu              sync.RWMutex
	services        map[string]interface{}
	factories       map[string]func() (interface{}, error)
	singletons      map[string]interface{}
	logger          *logrus.Logger
	datastoreClient *datastore.Client
	cacheManager    *cache.CacheManager
}

// NewContainer creates a new dependency injection container
func NewContainer() *Container {
	return &Container{
		services:   make(map[string]interface{}),
		factories:  make(map[string]func() (interface{}, error)),
		singletons: make(map[string]interface{}),
	}
}

// Register registers a service instance
func (c *Container) Register(name string, service interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.services[name] = service
}

// RegisterFactory registers a factory function for lazy service creation
func (c *Container) RegisterFactory(name string, factory func() (interface{}, error)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.factories[name] = factory
}

// RegisterSingleton registers a singleton service
func (c *Container) RegisterSingleton(name string, service interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.singletons[name] = service
}

// Get retrieves a service by name
func (c *Container) Get(name string) (interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Check if service is already registered
	if service, exists := c.services[name]; exists {
		return service, nil
	}

	// Check if it's a singleton
	if singleton, exists := c.singletons[name]; exists {
		return singleton, nil
	}

	// Check if there's a factory for this service
	if factory, exists := c.factories[name]; exists {
		service, err := factory()
		if err != nil {
			return nil, fmt.Errorf("failed to create service %s: %v", name, err)
		}
		return service, nil
	}

	return nil, fmt.Errorf("service %s not found", name)
}

// GetLogger retrieves the logger service
func (c *Container) GetLogger() (*logrus.Logger, error) {
	service, err := c.Get("logger")
	if err != nil {
		return nil, err
	}
	logger, ok := service.(*logrus.Logger)
	if !ok {
		return nil, fmt.Errorf("logger service is not of expected type")
	}
	return logger, nil
}

// GetDatastoreClient retrieves the datastore client service
func (c *Container) GetDatastoreClient() (*datastore.Client, error) {
	service, err := c.Get("datastore")
	if err != nil {
		return nil, err
	}
	client, ok := service.(*datastore.Client)
	if !ok {
		return nil, fmt.Errorf("datastore service is not of expected type")
	}
	return client, nil
}

// GetCacheManager retrieves the cache manager service
func (c *Container) GetCacheManager() (*cache.CacheManager, error) {
	service, err := c.Get("cache")
	if err != nil {
		return nil, err
	}
	cache, ok := service.(*cache.CacheManager)
	if !ok {
		return nil, fmt.Errorf("cache service is not of expected type")
	}
	return cache, nil
}

// GetHandler retrieves the handler service
func (c *Container) GetHandler() (*handlers.Handler, error) {
	service, err := c.Get("handler")
	if err != nil {
		return nil, err
	}
	handler, ok := service.(*handlers.Handler)
	if !ok {
		return nil, fmt.Errorf("handler service is not of expected type")
	}
	return handler, nil
}

// InitializeServices initializes all core services with proper dependencies
func (c *Container) InitializeServices(datastoreClient *datastore.Client, cacheManager *cache.CacheManager, logger *logrus.Logger) error {
	// Register core services
	c.RegisterSingleton("logger", logger)
	c.RegisterSingleton("datastore", datastoreClient)
	c.RegisterSingleton("cache", cacheManager)

	// Register handler factory that depends on other services
	c.RegisterFactory("handler", func() (interface{}, error) {
		return handlers.NewHandler(datastoreClient, cacheManager, logger), nil
	})

	return nil
}

// Close gracefully closes all service connections
func (c *Container) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Close datastore client if available
	if datastoreClient, err := c.GetDatastoreClient(); err == nil && datastoreClient != nil {
		if err := datastoreClient.Close(); err != nil {
			return fmt.Errorf("failed to close datastore client: %v", err)
		}
	}

	return nil
}
