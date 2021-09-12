package main

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	redis "github.com/go-redis/redis/v8"
)

// ICacher is the interface for cache service
type ICacher interface {
	Autonumber(name string) (int, error)
	Autonumbers(name string, n int) ([]int, error)

	BitFieldBulkUpdate(cmds []*BitFieldCmd) error
	BitField(key string, cmds []*BitFieldCmd) ([]int64, error)
	BitFieldGet(key string, byteSize int, position int) (int64, error)
	BitFieldSet(key string, byteSize int, position int, value interface{}) (int64, error)
	BitFieldIncrBy(key string, byteSize int, position int, value int64) (int64, error)

	HScan(key string, cursor uint64, fieldPattern string, count int64) ([]string, uint64 /*next cursor*/, error)
	HSetS(key string, field string, value string, expire time.Duration) error
	HSetSNoExpire(key string, field string, value string) error
	HIncrBy(key string, field string, val int) (int, error)
	HDecrBy(key string, field string, val int) (int, error)
	HIncr(key string, field string) (int, error)
	HDecr(key string, field string) (int, error)
	HMSet(key string, fieldValues map[string]interface{}) error
	HGet(key string, field string) (string, error)
	HMGet(key string, fields []string) ([]interface{}, error)
	HDel(key string, fields ...string) error
	HExists(key string, field string) (bool, error)
	HFields(key string, pattern string) ([]string, error)

	Set(key string, value interface{}, expire time.Duration) error
	SetS(key string, value string, expire time.Duration) error
	SetNoExpire(key string, value interface{}) error
	SetSNoExpire(key string, value string) error
	IncrBy(key string, val int) (int, error)
	DecrBy(key string, val int) (int, error)
	Incr(key string) (int, error)
	Decr(key string) (int, error)
	MSet(kv map[string]interface{}) error
	Get(key string) (string, error)
	MGet(keys []string) ([]interface{}, error)
	Expire(key string, expire time.Duration) error
	Expires(keys []string, expire time.Duration) error
	Del(keys ...string) error
	Exists(key string) (bool, error)

	Pub(channel string, message interface{}) error
	Sub(channels ...string) (<-chan *redis.Message, string /*subID used for close*/, error)
	Unsub(subID string) error

	Close() error

	// Keys might return value that match the pattern, because it use HScan internally
	Keys(pattern string) ([]string, error)
}

// ICacherConfig is cacher configuration interface
type ICacherConfig interface {
	Endpoint() string
	Password() string
	DB() int
	ConnectionSettings() ICacherConnectionSettings
}

// ICacherConnectionSettings is connection settings for cacher
type ICacherConnectionSettings interface {
	PoolSize() int
	MinIdleConns() int
	MaxRetries() int
	MinRetryBackoff() time.Duration
	MaxRetryBackoff() time.Duration
	IdleTimeout() time.Duration
	IdleCheckFrequency() time.Duration
	PoolTimeout() time.Duration
	ReadTimeout() time.Duration
	WriteTimeout() time.Duration
}

// DefaultCacherConnectionSettings contains default connection settings, this intend to use as embed struct
type DefaultCacherConnectionSettings struct{}

func NewDefaultCacherConnectionSettings() ICacherConnectionSettings {
	return &DefaultCacherConnectionSettings{}
}

func (setting *DefaultCacherConnectionSettings) PoolSize() int {
	return 50
}

func (setting *DefaultCacherConnectionSettings) MinIdleConns() int {
	return 5
}

func (setting *DefaultCacherConnectionSettings) MaxRetries() int {
	return 3
}

func (setting *DefaultCacherConnectionSettings) MinRetryBackoff() time.Duration {
	return 10 * time.Millisecond
}

func (setting *DefaultCacherConnectionSettings) MaxRetryBackoff() time.Duration {
	return 500 * time.Millisecond
}

func (setting *DefaultCacherConnectionSettings) IdleTimeout() time.Duration {
	return 30 * time.Minute
}

func (setting *DefaultCacherConnectionSettings) IdleCheckFrequency() time.Duration {
	return time.Minute
}

func (setting *DefaultCacherConnectionSettings) PoolTimeout() time.Duration {
	return time.Minute
}

func (setting *DefaultCacherConnectionSettings) ReadTimeout() time.Duration {
	return time.Minute
}

func (setting *DefaultCacherConnectionSettings) WriteTimeout() time.Duration {
	return time.Minute
}

type pubsubChannels struct {
	ps       *redis.PubSub
	channels []string
}

// Cacher is the struct for cache service
type Cacher struct {
	config      ICacherConfig
	clientMutex sync.Mutex
	client      *redis.Client
	oldClients  []*redis.Client
	subsribers  *sync.Map
	serviceID   int
}

// NewCacher return new Cacher
func NewCacher(config ICacherConfig) *Cacher {
	return &Cacher{
		config:     config,
		oldClients: nil,
		subsribers: &sync.Map{},
	}
}

func (cache *Cacher) newClient() *redis.Client {
	cfg := cache.config
	settings := cfg.ConnectionSettings()
	return redis.NewClient(&redis.Options{
		Addr:               cfg.Endpoint(),
		Password:           cfg.Password(),
		DB:                 cfg.DB(),
		PoolSize:           settings.PoolSize(),
		MinIdleConns:       settings.MinIdleConns(),
		MaxRetries:         settings.MaxRetries(),
		MinRetryBackoff:    settings.MinRetryBackoff(),
		MaxRetryBackoff:    settings.MaxRetryBackoff(),
		IdleTimeout:        settings.IdleTimeout(),
		IdleCheckFrequency: settings.IdleCheckFrequency(),
		PoolTimeout:        settings.PoolTimeout(),
		ReadTimeout:        settings.ReadTimeout(),
		WriteTimeout:       settings.WriteTimeout(),
	})
}

func (cache *Cacher) getClient() (*redis.Client, error) {
	cache.clientMutex.Lock()
	defer cache.clientMutex.Unlock()

	retriesDelayMs := cache.getRetriesDelayInMs()
	retries := -1
	for {
		retries++
		if retries > len(retriesDelayMs)-1 {
			return nil, fmt.Errorf("cacher: retry exceed limits")
		}

		client := cache.client
		if client == nil {
			client = cache.newClient()
			cache.client = client
		}

		_, err := client.Ping(context.Background()).Result()
		if err != nil {
			// Wait by retry delay then reset client and try connect again
			time.Sleep(time.Millisecond * time.Duration(retriesDelayMs[retries]))
			cache.client = nil
			continue
		}

		// If we can PING without error, just return
		return client, nil
	}
}

// Close close the redis client
func (cache *Cacher) Close() error {
	cache.clientMutex.Lock()
	defer cache.clientMutex.Unlock()

	// Close current client
	client := cache.client
	if client != nil {
		cache.client = nil

		err := client.Close()
		if err != nil {
			return err
		}

		// Close old clients
		for _, client := range cache.oldClients {
			err := client.Close()
			if err != nil {
				return err
			}
		}
		if len(cache.oldClients) > 0 {
			cache.oldClients = nil
		}
	}

	return nil
}

// Keys returns keys by given pattern
func (cache *Cacher) Keys(pattern string) ([]string, error) {

	c, err := cache.getClient()
	if err != nil {
		return nil, err
	}

	allKeys := map[string]interface{}{}

	var nextCursor uint64
	var keys []string

	retryLimit := 3
	for {
		retryLimit--
		if retryLimit < 0 {
			return nil, err
		}

		keys, nextCursor, err = c.Scan(context.Background(), 0, pattern, 100).Result()
		if err != nil {
			continue
		}
		// Scan can return duplidate item, so we use map to collect result set
		for _, key := range keys {
			allKeys[key] = struct{}{}
		}

		break // break retryLimit
	}

	for {
		if nextCursor == 0 {
			break
		}

		retryLimit := 3
		for {

			retryLimit--
			if retryLimit < 0 {
				return nil, err
			}

			keys, nextCursor, err = c.Scan(context.Background(), nextCursor, pattern, 100).Result()
			if err != nil {
				continue
			}

			// Scan can return duplidate item, so we use map to collect result set
			for _, key := range keys {
				allKeys[key] = struct{}{}
			}

			break // retryLimit
		}

	}

	retKeys := []string{}
	for key := range allKeys {
		retKeys = append(retKeys, key)
	}
	return retKeys, nil
}

// getRetriesDelayInMs sum only 1 second
func (cache *Cacher) getRetriesDelayInMs() []int {
	return []int{200, 200, 200, 200, 200}
}

func (cache *Cacher) isNoConnectionError(err error) bool {
	if err == nil {
		return false
	}

	errMsgLower := strings.ToLower(err.Error())
	if strings.Contains(errMsgLower, "connect: connection refused") {
		return true
	}

	return false
}

// Exists check if key is exists
func (cache *Cacher) Exists(key string) (bool, error) {

	c, err := cache.getClient()
	if err != nil {
		return false, err
	}

	val, err := c.Exists(context.Background(), key).Result()
	if err != nil {
		return false, err
	}

	// val == 1 means key is exists
	return val == 1, nil
}

// Del the cache by keys
func (cache *Cacher) Del(keys ...string) error {
	if len(keys) == 0 {
		return nil
	}

	c, err := cache.getClient()
	if err != nil {
		return err
	}

	// Delete 10000 items per page
	pageLimit := 10000
	from := 0
	to := pageLimit

	for {
		// Lower bound
		if from >= len(keys) {
			break
		}
		// Upper bound
		if to > len(keys) {
			to = len(keys)
		}

		delKeys := keys[from:to]
		if len(delKeys) == 0 {
			break
		}

		_, err = c.Del(context.Background(), delKeys...).Result()
		if err != nil {
			if err == redis.Nil {
				continue
			} else {
				return err
			}
		}
		from += pageLimit
		to += pageLimit
	}

	return nil
}

// Expires set expiration for objects in cache
// if there is error happen, just return last error
func (cache *Cacher) Expires(keys []string, expire time.Duration) error {
	return cache.expires(keys, expire)
}

// Expire set expiration for object in cache
func (cache *Cacher) Expire(key string, expire time.Duration) error {
	return cache.expires([]string{key}, expire)
}

// Expires set expiration for objects in cache
// if there is error happen, just return last error
func (cache *Cacher) expires(keys []string, expire time.Duration) error {
	c, err := cache.getClient()
	if err != nil {
		return err
	}

	var lastErr error
	for _, key := range keys {
		err = c.Expire(context.Background(), key, expire).Err()
		if err != nil {
			if err == redis.Nil {
				// Key does not exists
				return nil
			} else {
				lastErr = err
			}
		}
	}
	return lastErr
}

// MGet get by multiple keys, the value can be nil, so it will return []interface{} instead of []string
func (cache *Cacher) MGet(keys []string) ([]interface{}, error) {

	c, err := cache.getClient()
	if err != nil {
		return nil, err
	}

	vals, err := c.MGet(context.Background(), keys...).Result()
	if err == redis.Nil {
		// Key does not exists
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	return vals, nil
}

// Get object from cache
func (cache *Cacher) Get(key string) (string, error) {

	c, err := cache.getClient()
	if err != nil {
		return "", err
	}

	val, err := c.Get(context.Background(), key).Result()
	if err == redis.Nil {
		// Key does not exists
		return "", nil
	} else if err != nil {
		return "", err
	}

	return val, nil
}

// MSet set multiple key value
func (cache *Cacher) MSet(kv map[string]interface{}) error {

	c, err := cache.getClient()
	if err != nil {
		return err
	}

	pairs := []interface{}{}
	for k, v := range kv {

		str, ok := v.(string)
		// Check empty string if value string
		if ok && len(str) == 0 {
			pairs = append(pairs, k, "")
			continue
		}
		// If value is string, not pass it to json.Marshal
		if len(str) > 0 {
			pairs = append(pairs, k, str)
			continue
		}

		strb, err := json.Marshal(v)
		if err != nil {
			return err
		}
		pairs = append(pairs, k, strb)
	}

	err = c.MSet(context.Background(), pairs...).Err()
	if err != nil {
		return err
	}

	return nil
}

// Decr minus 1 to a counter on key, return first counter (-1) if cache expire
func (cache *Cacher) Decr(key string) (int, error) {

	c, err := cache.getClient()
	if err != nil {
		return 0, err
	}

	val, err := c.Decr(context.Background(), key).Result()
	if err == redis.Nil {
		// Key does not exists
		return 0, nil
	} else if err != nil {
		return 0, err
	}

	return int(val), nil
}

// Incr do a counter on key, return first counter if cache expire
func (cache *Cacher) Incr(key string) (int, error) {

	c, err := cache.getClient()
	if err != nil {
		return 0, err
	}

	val, err := c.Incr(context.Background(), key).Result()
	if err == redis.Nil {
		// Key does not exists
		return 0, nil
	} else if err != nil {
		return 0, err
	}

	return int(val), nil
}

// decrBy decrement the value on key by given value, return first -value if cache expire
func (cache *Cacher) DecrBy(key string, value int) (int, error) {

	c, err := cache.getClient()
	if err != nil {
		return 0, err
	}

	val, err := c.DecrBy(context.Background(), key, int64(value)).Result()
	if err == redis.Nil {
		// Key does not exists
		return 0, nil
	} else if err != nil {
		return 0, err
	}

	return int(val), nil
}

// IncrBy increment the value on key by given value, return first value if cache expire
func (cache *Cacher) IncrBy(key string, value int) (int, error) {

	c, err := cache.getClient()
	if err != nil {
		return 0, err
	}

	val, err := c.IncrBy(context.Background(), key, int64(value)).Result()
	if err == redis.Nil {
		// Key does not exists
		return 0, nil
	} else if err != nil {
		return 0, err
	}

	return int(val), nil
}

// SetSNoExpire set value as string into cache no expired
func (cache *Cacher) SetSNoExpire(key string, value string) error {

	c, err := cache.getClient()
	if err != nil {
		return err
	}

	// 0 = no expired
	err = c.Set(context.Background(), key, value, 0).Err()
	if err != nil {
		if err == redis.Nil {
			// Key does not exists
			return nil
		} else {
			return err
		}
	}

	return nil
}

// SetNoExpire set object into cache no expired
func (cache *Cacher) SetNoExpire(key string, value interface{}) error {

	c, err := cache.getClient()
	if err != nil {
		return err
	}

	str, err := json.Marshal(value)
	if err != nil {
		return err
	}

	// 0 = no expired
	err = c.Set(context.Background(), key, str, 0).Err()
	if err != nil {
		if err == redis.Nil {
			// Key does not exists
			return nil
		} else {
			return err
		}
	}

	return nil
}

// SetS set string into cache
func (cache *Cacher) SetS(key string, value string, expire time.Duration) error {

	c, err := cache.getClient()
	if err != nil {
		return err
	}

	err = c.Set(context.Background(), key, value, expire).Err()
	if err != nil {
		return err
	}

	return nil
}

func (cache *Cacher) Set(key string, value interface{}, expire time.Duration) error {

	c, err := cache.getClient()
	if err != nil {
		return err
	}

	str, err := json.Marshal(value)
	if err != nil {
		return err
	}

	err = c.Set(context.Background(), key, str, expire).Err()
	if err != nil {
		if err == redis.Nil {
			// Key does not exists
			return nil
		} else {
			return err
		}
	}

	return nil
}

func (cache *Cacher) HScan(
	key string, cursor uint64, fieldPattern string, count int64) ([]string, uint64 /*next cursor*/, error) {

	c, err := cache.getClient()
	if err != nil {
		return nil, 0, err
	}

	fields, nextCursor, err := c.HScan(context.Background(), key, cursor, fieldPattern, count).Result()
	if err != nil {
		return nil, 0, err
	}

	return fields, nextCursor, nil
}

func (cache *Cacher) HFields(key string, pattern string) ([]string, error) {

	c, err := cache.getClient()
	if err != nil {
		return nil, err
	}

	allFields := map[string]interface{}{}
	var nextCursor uint64
	var fields []string

	retryLimit := 3
	for {
		retryLimit--
		if retryLimit < 0 {
			return nil, err
		}
		fields, nextCursor, err = c.HScan(context.Background(), key, 0, pattern, 100).Result()
		if err != nil {
			continue
		}

		// Scan can return duplidate item, so we use map to collect result set
		for i, field := range fields {
			if i%2 == 0 {
				allFields[field] = struct{}{}
			}
		}
		break // retryLimit
	}

	for {
		if nextCursor == 0 {
			break
		}

		retryLimit := 3
		for {
			retryLimit--
			if retryLimit < 0 {
				return nil, err
			}

			fields, nextCursor, err = c.HScan(context.Background(), key, nextCursor, pattern, 100).Result()
			if err != nil {
				continue
			}

			// Scan can return duplidate item, so we use map to collect result set
			for i, field := range fields {
				if i%2 == 0 {
					allFields[field] = struct{}{}
				}
			}

			break // retryLimit
		}
	}

	retFields := []string{}
	for field := range allFields {
		retFields = append(retFields, field)
	}
	return retFields, nil
}

// HExists check if key is exists
func (cache *Cacher) HExists(key string, field string) (bool, error) {

	c, err := cache.getClient()
	if err != nil {
		return false, err
	}

	val, err := c.HExists(context.Background(), key, field).Result()
	if err != nil {
		if err == redis.Nil {
			// Key does not exists
			return false, nil
		} else {
			return false, err
		}
	}

	return val, nil
}

// Del the cache by keys
func (cache *Cacher) HDel(key string, fields ...string) error {

	c, err := cache.getClient()
	if err != nil {
		return err
	}

	_, err = c.HDel(context.Background(), key, fields...).Result()
	if err != nil {
		if err == redis.Nil {
			// Key does not exists
			return nil
		} else {
			return err
		}
	}

	return nil
}

// HGet object from cache
func (cache *Cacher) HGet(key string, field string) (string, error) {

	c, err := cache.getClient()
	if err != nil {
		return "", err
	}

	val, err := c.HGet(context.Background(), key, field).Result()
	if err == redis.Nil {
		// Key does not exists
		return "", nil
	} else if err != nil {
		return "", err
	}

	return val, nil
}

// HMGet get by multiple keys, the value can be nil, so it will return []interface{} instead of []string
func (cache *Cacher) HMGet(key string, fields []string) ([]interface{}, error) {

	c, err := cache.getClient()
	if err != nil {
		return nil, err
	}

	vals, err := c.HMGet(context.Background(), key, fields...).Result()
	if err == redis.Nil {
		// Key does not exists
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	return vals, nil
}

// HMSet set multiple key value
func (cache *Cacher) HMSet(key string, fieldValues map[string]interface{}) error {

	c, err := cache.getClient()
	if err != nil {
		return err
	}

	err = c.HMSet(context.Background(), key, fieldValues).Err()
	if err != nil {
		if err == redis.Nil {
			// Key does not exists
			return nil
		} else {
			return err
		}
	}

	return nil
}

// HDecr minus 1 to a counter on key, return first counter (-1) if cache expire
func (cache *Cacher) HDecr(key string, field string) (int, error) {

	c, err := cache.getClient()
	if err != nil {
		return 0, err
	}

	val, err := c.HIncrBy(context.Background(), key, field, -1).Result()
	if err == redis.Nil {
		// Key does not exists
		return 0, nil
	} else if err != nil {
		return 0, err
	}

	return int(val), nil
}

// Incr do a counter on key, return first counter if cache expire
func (cache *Cacher) HIncr(key string, field string) (int, error) {

	c, err := cache.getClient()
	if err != nil {
		return 0, err
	}

	val, err := c.HIncrBy(context.Background(), key, field, 1).Result()
	if err == redis.Nil {
		// Key does not exists
		return 0, nil
	} else if err != nil {
		return 0, err
	}

	return int(val), nil
}

// HDecrBy decrement the value on key by given value, return first -value if cache expire
func (cache *Cacher) HDecrBy(key string, field string, value int) (int, error) {

	c, err := cache.getClient()
	if err != nil {
		return 0, err
	}

	val, err := c.HIncrBy(context.Background(), key, field, -1*int64(value)).Result()
	if err == redis.Nil {
		// Key does not exists
		return 0, nil
	} else if err != nil {
		return 0, err
	}

	return int(val), nil
}

// HIncrBy increment the value on key by given value, return first value if cache expire
func (cache *Cacher) HIncrBy(key string, field string, value int) (int, error) {

	c, err := cache.getClient()
	if err != nil {
		return 0, err
	}

	val, err := c.HIncrBy(context.Background(), key, field, int64(value)).Result()
	if err == redis.Nil {
		// Key does not exists
		return 0, nil
	} else if err != nil {
		return 0, err
	}

	return int(val), nil
}

// HSetSNoExpire set value as string into cache no expired
func (cache *Cacher) HSetSNoExpire(key string, field string, value string) error {

	c, err := cache.getClient()
	if err != nil {
		return err
	}

	err = c.HSet(context.Background(), key, field, value).Err()
	if err != nil {
		return err
	}

	return nil
}

// HSetS set string into cache
func (cache *Cacher) HSetS(key string, field string, value string, expire time.Duration) error {

	c, err := cache.getClient()
	if err != nil {
		return err
	}

	err = c.HSet(context.Background(), key, field, value).Err()
	if err != nil {
		return err
	}

	if expire > 0 {
		err := cache.Expires([]string{key}, expire)
		if err != nil {
			if err == redis.Nil {
				// Key does not exists
				return nil
			} else {
				return err
			}
		}
	}

	return nil
}

func (cache *Cacher) BitFieldBulkUpdate(cmds []*BitFieldCmd) error {
	if len(cmds) == 0 {
		return nil
	}

	keyCmds := make(map[string][]*BitFieldCmd)
	keyExpires := make(map[string]time.Duration)

	for _, cmd := range cmds {
		if len(cmd.CacheKey) == 0 {
			continue
		}
		cmdArr, ok := keyCmds[cmd.CacheKey]
		if !ok {
			cmdArr = make([]*BitFieldCmd, 0)
		}
		cmdArr = append(cmdArr, cmd)

		keyCmds[cmd.CacheKey] = cmdArr
		if cmd.CacheExpire > 0 {
			keyExpires[cmd.CacheKey] = cmd.CacheExpire
		}
	}

	var err error
	for key, val := range keyCmds {

		_, err = cache.BitField(key, val)

		expire, ok := keyExpires[key]
		if ok && expire > 0 {
			cache.Expire(key, expire)
		}
	}

	return err
}

func (cache *Cacher) BitFieldGet(key string, byteSize int, position int) (int64, error) {
	cmd := NewBitFieldCmdGetU(byteSize, position)
	ress, err := cache.bitfield(key, []*BitFieldCmd{cmd})
	if err != nil {
		return 0, err
	}
	return ress[0], nil
}

func (cache *Cacher) BitFieldSet(key string, byteSize int, position int, value interface{}) (int64, error) {
	cmd := NewBitFieldCmdSetU(byteSize, position, value)
	ress, err := cache.bitfield(key, []*BitFieldCmd{NewBitFieldCmdOverflowSat(), cmd})
	if err != nil {
		return 0, err
	}
	return ress[0], nil
}

func (cache *Cacher) BitFieldIncrBy(key string, byteSize int, position int, value int64) (int64, error) {
	cmd := NewBitFieldCmdIncrByU(byteSize, position, value)
	ress, err := cache.bitfield(key, []*BitFieldCmd{NewBitFieldCmdOverflowSat(), cmd})
	if err != nil {
		return 0, err
	}
	return ress[0], nil
}

func (cache *Cacher) BitField(
	key string,
	cmds []*BitFieldCmd) ([]int64, error) {

	return cache.bitfield(key, cmds)
}

func (cache *Cacher) bitfield(
	key string,
	cmds []*BitFieldCmd) ([]int64, error) {

	if len(cmds) == 0 {
		return nil, nil
	}

	c, err := cache.getClient()
	if err != nil {
		return nil, err
	}

	args := []interface{}{}
	for _, cmd := range cmds {

		switch cmd.CmdType {
		case BitFieldCmdTypeGet:
			sign := "u"
			if cmd.Sign {
				sign = "i"
			}
			itemPosition := fmt.Sprintf("#%d", cmd.ItemPosition)
			byteSize := fmt.Sprintf("%s%d", sign, cmd.ByteSize)
			args = append(args, string(cmd.CmdType), byteSize, itemPosition)
		case BitFieldCmdTypeOverflow:
			args = append(args, string(cmd.CmdType), string(cmd.OverflowType))
		default:
			sign := "u"
			if cmd.Sign {
				sign = "i"
			}
			itemPosition := fmt.Sprintf("#%d", cmd.ItemPosition)
			byteSize := fmt.Sprintf("%s%d", sign, cmd.ByteSize)
			valStr := fmt.Sprintf("%d", cmd.Value)
			args = append(args, string(cmd.CmdType), byteSize, itemPosition, valStr)
		}
	}

	res, err := c.BitField(context.Background(), key, args...).Result()
	if err != nil {
		return nil, err
	}

	return res, nil
}

type BitFieldCmdType string

const (
	BitFieldCmdTypeIncrBy   BitFieldCmdType = "INCRBY"
	BitFieldCmdTypeGet      BitFieldCmdType = "GET"
	BitFieldCmdTypeSet      BitFieldCmdType = "SET"
	BitFieldCmdTypeOverflow BitFieldCmdType = "OVERFLOW"
)

type BitFieldOverflowType string

const (
	BitFieldOverflowTypeWrap BitFieldOverflowType = "WRAP"
	BitFieldOverflowTypeSat  BitFieldOverflowType = "SAT"
	BitFieldOverflowTypeFail BitFieldOverflowType = "FAIL"
)

type BitFieldCmd struct {
	CacheKey     string
	CacheExpire  time.Duration
	CmdType      BitFieldCmdType
	OverflowType BitFieldOverflowType
	ByteSize     int
	Sign         bool // false=unsigned integer, true=signed integer, default is unsigned
	ItemPosition int
	Value        interface{}
}

func (cmd *BitFieldCmd) SetCacheKey(cacheKey string) *BitFieldCmd {
	cmd.CacheKey = cacheKey
	return cmd
}

func (cmd *BitFieldCmd) SetCacheKeyWithExpire(cacheKey string, expire time.Duration) *BitFieldCmd {
	cmd.CacheKey = cacheKey
	cmd.CacheExpire = expire
	return cmd
}

type BitFieldCmdBuilder struct {
	cmds []*BitFieldCmd
}

func NewBitFieldCmdBuilderWithOverflow(firstOverflowType BitFieldOverflowType) *BitFieldCmdBuilder {
	overflow := NewBitFieldCmdOverflowSat()
	switch firstOverflowType {
	case BitFieldOverflowTypeWrap:
		overflow = NewBitFieldCmdOverflowWrap()
	case BitFieldOverflowTypeSat:
		overflow = NewBitFieldCmdOverflowSat()
	case BitFieldOverflowTypeFail:
		overflow = NewBitFieldCmdOverflowFail()
	}

	return &BitFieldCmdBuilder{
		cmds: []*BitFieldCmd{overflow},
	}
}

func NewBitFieldCmdBuilder() *BitFieldCmdBuilder {
	return &BitFieldCmdBuilder{
		cmds: []*BitFieldCmd{},
	}
}

func (builder *BitFieldCmdBuilder) AddCommand(cmd *BitFieldCmd) {
	builder.cmds = append(builder.cmds, cmd)
}

func (builder *BitFieldCmdBuilder) AddCommandByKey(cacheKey string, cmd *BitFieldCmd) {
	cmd.CacheKey = cacheKey
	cmd.CacheExpire = 0
	builder.cmds = append(builder.cmds, cmd)
}

func (builder *BitFieldCmdBuilder) AddCommandByKeyExpired(cacheKey string, expire time.Duration, cmd *BitFieldCmd) {
	cmd.CacheKey = cacheKey
	cmd.CacheExpire = expire
	builder.cmds = append(builder.cmds, cmd)
}

func (builder *BitFieldCmdBuilder) Commands() []*BitFieldCmd {
	return builder.cmds
}

func (builder *BitFieldCmdBuilder) Size() int {
	return len(builder.cmds)
}

func NewBitFieldCmdOverflowWrap() *BitFieldCmd {
	return &BitFieldCmd{
		CmdType:      BitFieldCmdTypeOverflow,
		OverflowType: BitFieldOverflowTypeWrap,
	}
}

func NewBitFieldCmdOverflowSat() *BitFieldCmd {
	return &BitFieldCmd{
		CmdType:      BitFieldCmdTypeOverflow,
		OverflowType: BitFieldOverflowTypeSat,
	}
}

func NewBitFieldCmdOverflowFail() *BitFieldCmd {
	return &BitFieldCmd{
		CmdType:      BitFieldCmdTypeOverflow,
		OverflowType: BitFieldOverflowTypeFail,
	}
}

func NewBitFieldCmdGetU(byteSize int, itemPosition int) *BitFieldCmd {
	return &BitFieldCmd{
		CmdType:      BitFieldCmdTypeGet,
		ByteSize:     byteSize,
		ItemPosition: itemPosition,
		Value:        0,
	}
}

func NewBitFieldCmdGetU1(itemPosition int) *BitFieldCmd {
	return NewBitFieldCmdGetU(1, itemPosition)
}

func NewBitFieldCmdGetU2(itemPosition int) *BitFieldCmd {
	return NewBitFieldCmdGetU(2, itemPosition)
}

func NewBitFieldCmdGetU3(itemPosition int) *BitFieldCmd {
	return NewBitFieldCmdGetU(3, itemPosition)
}

func NewBitFieldCmdGetU4(itemPosition int) *BitFieldCmd {
	return NewBitFieldCmdGetU(4, itemPosition)
}

func NewBitFieldCmdGetU5(itemPosition int) *BitFieldCmd {
	return NewBitFieldCmdGetU(5, itemPosition)
}

func NewBitFieldCmdGetU6(itemPosition int) *BitFieldCmd {
	return NewBitFieldCmdGetU(6, itemPosition)
}

func NewBitFieldCmdGetU7(itemPosition int) *BitFieldCmd {
	return NewBitFieldCmdGetU(7, itemPosition)
}

func NewBitFieldCmdGetU8(itemPosition int) *BitFieldCmd {
	return NewBitFieldCmdGetU(8, itemPosition)
}

func NewBitFieldCmdGetU16(itemPosition int) *BitFieldCmd {
	return NewBitFieldCmdGetU(16, itemPosition)
}

func NewBitFieldCmdGetU32(itemPosition int) *BitFieldCmd {
	return NewBitFieldCmdGetU(32, itemPosition)
}

func NewBitFieldCmdSetU(byteSize int, itemPosition int, value interface{}) *BitFieldCmd {
	return &BitFieldCmd{
		CmdType:      BitFieldCmdTypeSet,
		ByteSize:     byteSize,
		ItemPosition: itemPosition,
		Value:        value,
	}
}

func NewBitFieldCmdSetU1(itemPosition int, value interface{}) *BitFieldCmd {
	return NewBitFieldCmdSetU(1, itemPosition, value)
}

func NewBitFieldCmdSetU2(itemPosition int, value interface{}) *BitFieldCmd {
	return NewBitFieldCmdSetU(2, itemPosition, value)
}

func NewBitFieldCmdSetU3(itemPosition int, value interface{}) *BitFieldCmd {
	return NewBitFieldCmdSetU(3, itemPosition, value)
}

func NewBitFieldCmdSetU4(itemPosition int, value interface{}) *BitFieldCmd {
	return NewBitFieldCmdSetU(4, itemPosition, value)
}

func NewBitFieldCmdSetU5(itemPosition int, value interface{}) *BitFieldCmd {
	return NewBitFieldCmdSetU(5, itemPosition, value)
}

func NewBitFieldCmdSetU6(itemPosition int, value interface{}) *BitFieldCmd {
	return NewBitFieldCmdSetU(6, itemPosition, value)
}

func NewBitFieldCmdSetU7(itemPosition int, value interface{}) *BitFieldCmd {
	return NewBitFieldCmdSetU(7, itemPosition, value)
}

func NewBitFieldCmdSetU8(itemPosition int, value interface{}) *BitFieldCmd {
	return NewBitFieldCmdSetU(8, itemPosition, value)
}

func NewBitFieldCmdSetU16(itemPosition int, value interface{}) *BitFieldCmd {
	return NewBitFieldCmdSetU(16, itemPosition, value)
}

func NewBitFieldCmdSetU32(itemPosition int, value interface{}) *BitFieldCmd {
	return NewBitFieldCmdSetU(32, itemPosition, value)
}

func NewBitFieldCmdIncrByU(byteSize int, itemPosition int, value int64) *BitFieldCmd {
	return &BitFieldCmd{
		CmdType:      BitFieldCmdTypeIncrBy,
		ByteSize:     byteSize,
		ItemPosition: itemPosition,
		Value:        value,
	}
}

func NewBitFieldCmdIncrByU1(itemPosition int, value int64) *BitFieldCmd {
	return NewBitFieldCmdIncrByU(1, itemPosition, value)
}

func NewBitFieldCmdIncrByU2(itemPosition int, value int64) *BitFieldCmd {
	return NewBitFieldCmdIncrByU(2, itemPosition, value)
}

func NewBitFieldCmdIncrByU3(itemPosition int, value int64) *BitFieldCmd {
	return NewBitFieldCmdIncrByU(3, itemPosition, value)
}

func NewBitFieldCmdIncrByU4(itemPosition int, value int64) *BitFieldCmd {
	return NewBitFieldCmdIncrByU(4, itemPosition, value)
}

func NewBitFieldCmdIncrByU5(itemPosition int, value int64) *BitFieldCmd {
	return NewBitFieldCmdIncrByU(5, itemPosition, value)
}

func NewBitFieldCmdIncrByU6(itemPosition int, value int64) *BitFieldCmd {
	return NewBitFieldCmdIncrByU(6, itemPosition, value)
}

func NewBitFieldCmdIncrByU7(itemPosition int, value int64) *BitFieldCmd {
	return NewBitFieldCmdIncrByU(7, itemPosition, value)
}

func NewBitFieldCmdIncrByU8(itemPosition int, value int64) *BitFieldCmd {
	return NewBitFieldCmdIncrByU(8, itemPosition, value)
}

func NewBitFieldCmdIncrByU16(itemPosition int, value int64) *BitFieldCmd {
	return NewBitFieldCmdIncrByU(16, itemPosition, value)
}

func NewBitFieldCmdIncrByU32(itemPosition int, value int64) *BitFieldCmd {
	return NewBitFieldCmdIncrByU(32, itemPosition, value)
}

func NewBitFieldCmdU(cmdType BitFieldCmdType, byteSize int, itemPosition int, value int) *BitFieldCmd {
	return &BitFieldCmd{
		CmdType:      cmdType,
		ByteSize:     byteSize,
		ItemPosition: itemPosition,
		Value:        value,
	}
}

func NewBitFieldCmdU1(cmdType BitFieldCmdType, itemPosition int, value int) *BitFieldCmd {
	return NewBitFieldCmdU(cmdType, 1, itemPosition, value)
}

func NewBitFieldCmdU2(cmdType BitFieldCmdType, itemPosition int, value int) *BitFieldCmd {
	return NewBitFieldCmdU(cmdType, 2, itemPosition, value)
}

func NewBitFieldCmdU3(cmdType BitFieldCmdType, itemPosition int, value int) *BitFieldCmd {
	return NewBitFieldCmdU(cmdType, 3, itemPosition, value)
}

func NewBitFieldCmdU4(cmdType BitFieldCmdType, itemPosition int, value int) *BitFieldCmd {
	return NewBitFieldCmdU(cmdType, 4, itemPosition, value)
}

func NewBitFieldCmdU5(cmdType BitFieldCmdType, itemPosition int, value int) *BitFieldCmd {
	return NewBitFieldCmdU(cmdType, 5, itemPosition, value)
}

func NewBitFieldCmdU6(cmdType BitFieldCmdType, itemPosition int, value int) *BitFieldCmd {
	return NewBitFieldCmdU(cmdType, 6, itemPosition, value)
}

func NewBitFieldCmdU7(cmdType BitFieldCmdType, itemPosition int, value int) *BitFieldCmd {
	return NewBitFieldCmdU(cmdType, 7, itemPosition, value)
}

func NewBitFieldCmdU8(cmdType BitFieldCmdType, itemPosition int, value int) *BitFieldCmd {
	return NewBitFieldCmdU(cmdType, 8, itemPosition, value)
}

func NewBitFieldCmdU16(cmdType BitFieldCmdType, itemPosition int, value int) *BitFieldCmd {
	return NewBitFieldCmdU(cmdType, 16, itemPosition, value)
}

func NewBitFieldCmdU32(cmdType BitFieldCmdType, itemPosition int, value int) *BitFieldCmd {
	return NewBitFieldCmdU(cmdType, 32, itemPosition, value)
}

func (cache *Cacher) Autonumber(name string) (int, error) {
	key := fmt.Sprintf("autonumber_%s", name)
	nextNumber, err := cache.Incr(key)
	if err != nil {
		return -1, err
	}
	return nextNumber, nil
}

func (cache *Cacher) Autonumbers(name string, n int) ([]int, error) {
	if n < 0 {
		return nil, fmt.Errorf("n must greter than 0")
	}
	if n == 0 {
		return nil, nil
	}

	key := fmt.Sprintf("autonumber_%s", name)
	nextNumber, err := cache.IncrBy(key, n)
	if err != nil {
		return nil, err
	}
	ress := make([]int, n)
	for i := 0; i < n; i++ {
		ress[i] = nextNumber - (n - i) + 1
	}
	return ress, nil
}

// Pub will publish to subscriber
func (cache *Cacher) Pub(channel string, message interface{}) error {

	c, err := cache.getClient()
	if err != nil {
		return err
	}

	retriesDelayMs := cache.getRetriesDelayInMs()
	retries := -1
	for {
		retries++
		if retries > len(retriesDelayMs)-1 {
			return fmt.Errorf("cacher: retry exceed limits")
		}

		_, err = c.Publish(context.Background(), channel, message).Result()
		if err != nil {
			if cache.isNoConnectionError(err) {
				time.Sleep(time.Millisecond * time.Duration(retriesDelayMs[retries]))
				continue
			}
			return err
		}

		return nil
	}
}

// Sub subscribe to channel
func (cache *Cacher) Sub(channels ...string) (<-chan *redis.Message /*subID (used for close)*/, string, error) {

	c, err := cache.getClient()
	if err != nil {
		return nil, "", err
	}

	ps := c.Subscribe(context.Background(), channels...)
	subID := NewUUID()

	cache.subsribers.Store(subID, &pubsubChannels{
		ps:       ps,
		channels: channels,
	})

	return ps.Channel(), subID, nil
}

// Unsub will unsub subscriber
func (cache *Cacher) Unsub(subID string) error {
	if len(subID) == 0 {
		return nil
	}

	psChannels, ok := cache.subsribers.Load(subID)
	if !ok {
		return nil
	}
	pubsubChannels, ok := psChannels.(*pubsubChannels)
	if !ok {
		return nil
	}

	if pubsubChannels.ps != nil {
		err := pubsubChannels.ps.Unsubscribe(context.Background(), pubsubChannels.channels...)
		if err != nil {
			_, fn, line, _ := runtime.Caller(1)
			fmt.Println(err.Error(), fn, line)
		}
		err = pubsubChannels.ps.Close()
		if err != nil {
			_, fn, line, _ := runtime.Caller(1)
			fmt.Println(err.Error(), fn, line)
		}
	}

	cache.subsribers.Delete(subID)

	return nil
}
