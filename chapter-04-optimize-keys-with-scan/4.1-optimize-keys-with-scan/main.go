package main

import (
	"fmt"
	"net/http"
	"time"

	_ "github.com/3dsinteractive/wrkgo"
)

func main() {

	cfg := NewConfig()

	// 1. Create microservices
	ms := NewMicroservice()

	totalKeys := 300000

	// 2. Setup workshop by adding 300k keys into redis
	err := setup(cfg)
	if err != nil {
		ms.Log("Main", err.Error())
		return
	}

	// 3. Random cacheKey and return the value
	ms.GET("/api", func(ctx IContext) error {
		cacheKey := fmt.Sprintf("key::%d", RandomMinMax(0, totalKeys-1))
		cacher := ctx.Cacher(cfg.CacherConfig())
		val, err := cacher.Get(cacheKey)
		if err != nil {
			ctx.Response(http.StatusInternalServerError, map[string]interface{}{"status": "error"})
			return nil
		}

		resp := map[string]interface{}{
			"status": "ok",
			"val":    val,
		}
		ctx.Response(http.StatusOK, resp)
		return nil
	})

	// 4. Read KEYS parallel with the API
	// 4.1 Continue running read keys using KEYS command
	go func() {
		pattern := fmt.Sprintf("*%d*", RandomMinMax(0, 999))
		cacher := NewCacher(cfg.CacherConfig())
		i := 0
		for {
			i++
			ms.Log("KeysN", fmt.Sprintf("Read Keys round %d", i))
			_, err := cacher.KeysN(pattern)
			if err != nil {
				ms.Log("KeysN", "error: "+err.Error())
				continue
			}
			time.Sleep(250 * time.Millisecond)
		}
	}()

	// 4.2 Run read keys using Scan command
	// go func() {
	// 	pattern := fmt.Sprintf("*%d*", RandomMinMax(0, 999))
	// 	cacher := NewCacher(cfg.CacherConfig())
	// 	i := 0
	// 	for {
	// 		i++
	// 		ms.Log("Keys", fmt.Sprintf("Read Keys round %d", i))
	// 		_, err := cacher.Keys(pattern)
	// 		if err != nil {
	// 			ms.Log("Keys", "error: "+err.Error())
	// 			continue
	// 		}
	// 		time.Sleep(250 * time.Millisecond)
	// 	}
	// }()

	// 5. Cleanup when exit
	defer ms.Cleanup()
	ms.Start()
}
