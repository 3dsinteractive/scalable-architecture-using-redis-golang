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
		// input format {"country":"thailand"}
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

	// 4. Register popcat api use redis with buffering
	// buffer := map[string]int{} // buffer[country_name] => counter to update
	// bufferMutex := sync.Mutex{}

	// counters := map[string]int{} // counters[country_name] => cache counters to response
	// countersMutex := sync.Mutex{}

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

	// 	// increment local buffer counter
	// 	bufferMutex.Lock()
	// 	val, ok := buffer[country]
	// 	if ok {
	// 		buffer[country] = val + 1
	// 	} else {
	// 		buffer[country] = 1
	// 	}
	// 	bufferMutex.Unlock()

	// 	// read from cached counters
	// 	counter := counters[country]
	// 	resp := map[string]interface{}{
	// 		"status":  "ok",
	// 		"country": country,
	// 		"counter": counter,
	// 	}
	// 	ctx.Response(http.StatusOK, resp)
	// 	return nil
	// })

	// go func() {
	// 	t := time.NewTicker(time.Second * 1)
	// 	// Trigger update to cacher every 1 second
	// 	for range t.C {
	// 		// We don't want the change to the buffer while we are reading so we use mutex to lock
	// 		bufferMutex.Lock()
	// 		for country, counter := range buffer {
	// 			// ms.Log("Worker", fmt.Sprintf("update %s by %d", country, counter))
	// 			cacher := ms.Cacher(cfg.CacherConfig())
	// 			updatedCounter, err := increaseCounterBy(cacher, country, counter)
	// 			if err != nil {
	// 				ms.Log("Worker", "error: "+err.Error())
	// 				continue
	// 			}

	// 			// Kept the last updated counters for next api call to return
	// 			countersMutex.Lock()
	// 			counters[country] = updatedCounter
	// 			countersMutex.Unlock()
	// 		}

	// 		buffer = map[string]int{}
	// 		bufferMutex.Unlock()
	// 	}
	// }()

	// 5. Cleanup when exit
	defer ms.Cleanup()
	ms.Start()
}

func increaseCounter(ctx IContext, cfg IConfig, country string) (int /*counter*/, error) {
	cacher := ctx.Cacher(cfg.CacherConfig())
	cacheKey := countryCounterCacheKey(country)

	// return counter is the number after increment
	counter, err := cacher.Incr(cacheKey)
	if err != nil {
		return 0, err
	}
	return counter, nil
}

func increaseCounterBy(cacher ICacher, country string, counter int) (int /*counter*/, error) {
	cacheKey := countryCounterCacheKey(country)
	counter, err := cacher.IncrBy(cacheKey, counter)
	if err != nil {
		return 0, err
	}
	return counter, nil
}

func countryCounterCacheKey(country string) string {
	// key format "counter::thailand", "counter::japan"
	return fmt.Sprintf("counter::%s", country)
}
