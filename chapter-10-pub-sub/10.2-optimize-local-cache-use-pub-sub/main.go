package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	_ "github.com/3dsinteractive/wrkgo"
)

const channelClearCache = "channel::clear_cache"

func main() {

	cfg := NewConfig()

	// 1. Create microservices
	ms := NewMicroservice()

	// 2. Migrate and seed 100,000 members
	ms.Log("Main", "Migrate database...")
	err := setup(cfg)
	if err != nil {
		ms.Log("Main", err.Error())
		return
	}

	// levels is the local map to cache the latest update member levels
	levels := map[string]int{}
	levelsMutex := sync.RWMutex{}

	// 3. GET level api using cache and local memory
	ms.GET("/level", func(ctx IContext) error {
		username := ctx.QueryParam("u")

		// 1. Find in local memory cache
		levelsMutex.RLock()
		level, ok := levels[username]
		levelsMutex.RUnlock()
		// If found from local memory cache, response
		if ok {
			resp := map[string]interface{}{
				"status": "ok",
				"level":  level,
			}
			ctx.Response(http.StatusOK, resp)
			return nil
		}

		// 2. Find in redis cache
		cacheKey := getCacheKeyForMember(username)
		cacheField := "level"
		cacheTimeout := 300 * time.Second
		level = -1

		cacher := ctx.Cacher(cfg.CacherConfig())
		levelJS, err := cacher.HGet(cacheKey, cacheField)
		if err != nil {
			ctx.Log(err.Error())
		}

		if len(levelJS) > 0 {
			// ctx.Log("cache hit")
			level, err = strconv.Atoi(levelJS)
			if err != nil {
				level = -1
				cacher.HDel(cacheKey, cacheField)
				ctx.Log(err.Error())
			}

			// Set in local cache if found in redis cache
			levelsMutex.Lock()
			levels[username] = level
			levelsMutex.Unlock()
		}

		// 3. If miss all cache, find from database
		if level <= 0 {
			level, err = queryMemberLevel(ctx, cfg, username)
			if err != nil {
				ctx.Response(http.StatusInternalServerError, map[string]interface{}{"status": "error"})
				return nil
			}

			// Set result from the database in redis cache
			err = cacher.HSetS(cacheKey, cacheField, fmt.Sprintf("%d", level), cacheTimeout)
			if err != nil {
				ctx.Log(err.Error())
			}

			// Set result from the database in local memory cache
			levelsMutex.Lock()
			levels[username] = level
			levelsMutex.Unlock()
		}

		resp := map[string]interface{}{
			"status": "ok",
			"level":  level,
		}
		ctx.Response(http.StatusOK, resp)

		return nil
	})

	// 4. Start worker to subscribe channel to clear cache
	go func() {

		ms.Log("Subscriber", "Worker clear local cache is starting")

		cacher := ms.Cacher(cfg.CacherConfig())
		onClearCache, subID, err := cacher.Sub(channelClearCache)
		if err != nil {
			ms.Log("Subscriber", err.Error())
			return
		}

		osQuit := make(chan os.Signal, 1)
		signal.Notify(osQuit, syscall.SIGTERM, syscall.SIGINT)

		for {
			select {
			case msg := <-onClearCache:
				if msg == nil {
					// This happen when cacher close
					return
				}

				username := msg.Payload

				ms.Log("Subscriber", fmt.Sprintf("Clear cache for username %s", username))

				// Delete local member level from cache
				// when get the signal from publisher
				levelsMutex.Lock()
				delete(levels, username)
				levelsMutex.Unlock()

			case <-osQuit:
				// Unsub from channel
				err = cacher.Unsub(subID)
				if err != nil {
					ms.Log("Subscriber", err.Error())
				}
				os.Exit(0)
			}
		}
	}()

	// 5. API to update member level
	ms.PUT("/member/level", func(ctx IContext) error {
		input := ctx.ReadInput()
		payload := map[string]interface{}{}
		err := json.Unmarshal([]byte(input), &payload)
		if err != nil {
			ctx.Response(http.StatusOK, map[string]interface{}{"status": "invalid input"})
			return nil
		}

		username, _ := payload["username"].(string)
		newLevel, _ := payload["level"].(int)

		// 1. Update member level in the database
		//    and notify all subscriber that member level has changed
		err = updateLevel(ctx, cfg, username, newLevel)
		if err != nil {
			ctx.Response(http.StatusInternalServerError, map[string]interface{}{
				"status": "error",
				"error":  err.Error(),
			})
			return nil
		}

		resp := map[string]interface{}{
			"status": "ok",
		}
		ctx.Response(http.StatusOK, resp)

		return nil
	})

	// 5. Cleanup when exit
	defer ms.Cleanup()
	ms.Start()
}

func queryMemberLevel(ctx IContext, cfg IConfig, username string) (int /*level*/, error) {
	pst := ctx.Persister(cfg.PersisterConfig())
	members := make([]*Member, 0)
	_, err := pst.WhereP(
		&members,
		1, // limits
		1, // pages
		"username = ?",
		username)
	if err != nil {
		return -1, err
	}

	if len(members) == 0 {
		return -1, fmt.Errorf("member not found")
	}
	return members[0].MemberLevel, nil
}

func updateLevel(ctx IContext, cfg IConfig, username string, newLevel int) error {
	pst := ctx.Persister(cfg.PersisterConfig())
	members := make([]*Member, 0)
	_, err := pst.WhereP(
		&members,
		1, // limits
		1, // pages
		"username = ?",
		username)
	if err != nil {
		return err
	}

	if len(members) == 0 {
		return fmt.Errorf("member not found")
	}

	// 1. Update member level
	member := members[0]
	member.MemberLevel = newLevel
	err = pst.Update(member)
	if err != nil {
		return err
	}

	// 2. Clear level cache from redis
	cacher := ctx.Cacher(cfg.CacherConfig())
	cacheKey := getCacheKeyForMember(username)
	cacheField := "level"
	err = cacher.HDel(cacheKey, cacheField)
	if err != nil {
		return err
	}

	// 3. Notify all subsriber to delete local cache
	err = cacher.Pub(channelClearCache, username)
	if err != nil {
		return err
	}

	return nil
}

func getCacheKeyForMember(username string) string {
	return fmt.Sprintf("user::%s", username)
}
