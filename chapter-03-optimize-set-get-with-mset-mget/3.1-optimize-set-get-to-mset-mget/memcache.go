package main

import (
	"time"

	"github.com/3dsinteractive/go-cache"
)

// IMemCacher is the interface for in memory cache
type IMemCacher interface {
	Set(key string, value interface{}, expire time.Duration) error
	Get(key string) (interface{}, error)
	Del(key string) error

	MSet(items map[string]interface{}, expire time.Duration) error
	MGet(keys []string) ([]interface{}, error)
	MDel(keys []string) error
}

// MemCacher is the struct for in-memory cache service
type MemCacher struct {
	client *cache.Cache
}

// NewMemCacher return new in-memory Cacher
func NewMemCacher() *MemCacher {
	return &MemCacher{}
}

func (c *MemCacher) getClient() *cache.Cache {
	if c.client == nil {
		c.client = cache.New(3*time.Minute, 5*time.Second)
	}
	return c.client
}

// Set value for key
func (c *MemCacher) Set(key string, value interface{}, expire time.Duration) error {
	client := c.getClient()
	client.Set(key, value, expire)
	return nil
}

// Get return value for key
func (c *MemCacher) Get(key string) (interface{}, error) {
	client := c.getClient()
	val, found := client.Get(key)
	if found {
		return val, nil
	}
	return nil, nil
}

// Del delete value for key
func (c *MemCacher) Del(key string) error {
	client := c.getClient()
	client.Delete(key)
	return nil
}

func (c *MemCacher) MSet(items map[string]interface{}, expire time.Duration) error {
	var lastErr error
	for k, v := range items {
		err := c.Set(k, v, expire)
		if err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func (c *MemCacher) MGet(keys []string) ([]interface{}, error) {
	var lastErr error
	retVals := make([]interface{}, len(keys))
	for i, k := range keys {
		val, err := c.Get(k)
		if err != nil {
			lastErr = err
		}
		retVals[i] = val
	}
	return retVals, lastErr
}

func (c *MemCacher) MDel(keys []string) error {
	var lastErr error
	for _, k := range keys {
		err := c.Del(k)
		if err != nil {
			lastErr = err
		}
	}
	return lastErr
}
