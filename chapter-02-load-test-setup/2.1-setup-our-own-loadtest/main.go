package main

import (
	"net/http"
	"time"

	_ "github.com/3dsinteractive/wrkgo"
)

func main() {

	ms := NewMicroservice()

	ms.GET("/api", func(ctx IContext) error {

		// 1. external api (the slowest)
		// requestExternalAPI()

		// 2. database access (medium slow)
		// queryDatabase()

		// 3. cache access (the fastest)
		// queryCache()

		resp := map[string]string{
			"status": "ok",
		}
		ctx.Response(http.StatusOK, resp)
		return nil
	})

	defer ms.Cleanup()
	ms.Start()
}

func queryCache() {
	// Simulate the access cache block for 10ms
	time.Sleep(10 * time.Millisecond)
}

func queryDatabase() {
	// Simulate the access database block for 500ms
	time.Sleep(500 * time.Millisecond)
}

func requestExternalAPI() {
	// Simulate the external api request block for 1000ms
	time.Sleep(1000 * time.Millisecond)
}
