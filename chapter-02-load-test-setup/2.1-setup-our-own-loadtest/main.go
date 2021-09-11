package main

import (
	"net/http"

	_ "github.com/3dsinteractive/wrkgo"
)

func main() {

	ms := NewMicroservice()

	ms.GET("/api", func(ctx IContext) error {

		// Simulate the access database block for 500ms
		// time.Sleep(500 * time.Millisecond)

		// Simulate the access database block for 10ms
		// time.Sleep(10 * time.Millisecond)

		resp := map[string]string{
			"status": "ok",
		}
		ctx.Response(http.StatusOK, resp)
		return nil
	})

	defer ms.Cleanup()
	ms.Start()
}
