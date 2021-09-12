package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	_ "github.com/3dsinteractive/wrkgo"
)

func main() {

	cfg := NewConfig()

	// 1. Create microservices
	ms := NewMicroservice()

	// 2. Setup project
	err := setup(cfg)
	if err != nil {
		ms.Log("Main", err.Error())
		return
	}

	// 3. Register popcat api use redis
	ms.POST("/popcat", func(ctx IContext) error {
		input := ctx.ReadInput()
		payload := map[string]interface{}{}
		err := json.Unmarshal([]byte(input), &payload)
		if err != nil {
			ctx.Response(http.StatusOK, map[string]interface{}{
				"status": "invalid input",
				"error":  err.Error(),
			})
			return nil
		}

		country, ok := payload["country"].(string)
		if !ok {
			ctx.Response(http.StatusOK, map[string]interface{}{"status": "invalid input"})
			return nil
		}

		counter, err := increaseCounter(ctx, cfg, country)
		if err != nil {
			ctx.Response(http.StatusInternalServerError, map[string]interface{}{
				"status": "error",
				"error":  err.Error(),
			})
			return nil
		}

		resp := map[string]interface{}{
			"status":  "ok",
			"country": country,
			"counter": counter,
		}
		ctx.Response(http.StatusOK, resp)
		return nil
	})

	// 4. Register popcat api use redis with buffer
	// buffer := map[string]int{}
	// bufferMutex := sync.Mutex{}

	// counters := map[string]int{}
	// countersMutex := sync.Mutex{}

	// go func() {
	// 	t := time.NewTicker(time.Second * 1)
	// 	for range t.C {
	// 		bufferMutex.Lock()
	// 		for country, counter := range buffer {
	// 			// ms.Log("Worker", fmt.Sprintf("update %s by %d", country, counter))
	// 			updatedCounter, err := increaseCounterBy(cfg, country, counter)
	// 			if err != nil {
	// 				ms.Log("Worker", "error: "+err.Error())
	// 				continue
	// 			}

	// 			countersMutex.Lock()
	// 			counters[country] = updatedCounter
	// 			countersMutex.Unlock()
	// 		}

	// 		buffer = map[string]int{}
	// 		bufferMutex.Unlock()
	// 	}
	// }()

	// ms.POST("/popcat", func(ctx IContext) error {
	// 	input := ctx.ReadInput()
	// 	payload := map[string]interface{}{}
	// 	err := json.Unmarshal([]byte(input), &payload)
	// 	if err != nil {
	// 		ctx.Response(http.StatusOK, map[string]interface{}{
	// 			"status": "invalid input",
	// 			"error":  err.Error(),
	// 		})
	// 		return nil
	// 	}

	// 	country, ok := payload["country"].(string)
	// 	if !ok {
	// 		ctx.Response(http.StatusOK, map[string]interface{}{"status": "invalid input"})
	// 		return nil
	// 	}

	// 	bufferMutex.Lock()
	// 	val, ok := buffer[country]
	// 	if ok {
	// 		buffer[country] = val + 1
	// 	} else {
	// 		buffer[country] = 1
	// 	}
	// 	bufferMutex.Unlock()

	// 	counter := counters[country]
	// 	resp := map[string]interface{}{
	// 		"status":  "ok",
	// 		"country": country,
	// 		"counter": counter,
	// 	}
	// 	ctx.Response(http.StatusOK, resp)
	// 	return nil
	// })

	// 5. Cleanup when exit
	defer ms.Cleanup()
	ms.Start()
}

func increaseCounter(ctx IContext, cfg IConfig, country string) (int /*counter*/, error) {
	cacher := ctx.Cacher(cfg.CacherConfig())
	cacheKey := countryCounterCacheKey(country)

	counter, err := cacher.Incr(cacheKey)
	if err != nil {
		return 0, err
	}
	return counter, nil
}

var workerCacher ICacher

func increaseCounterBy(cfg IConfig, country string, counter int) (int /*counter*/, error) {
	cacher := workerCacher
	if cacher == nil {
		workerCacher = NewCacher(cfg.CacherConfig())
	}
	cacher = workerCacher

	cacheKey := countryCounterCacheKey(country)
	counter, err := cacher.IncrBy(cacheKey, counter)
	if err != nil {
		return 0, err
	}
	return counter, nil
}

func countryCounterCacheKey(country string) string {
	return fmt.Sprintf("counter::%s", country)
}
