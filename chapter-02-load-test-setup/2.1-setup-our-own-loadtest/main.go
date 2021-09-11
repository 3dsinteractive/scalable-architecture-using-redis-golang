package main

import (
	"net/http"
)

func main() {

	ms := NewMicroservice()

	ms.GET("/loadtest", func(ctx IContext) error {
		resp := map[string]string{
			"status": "ok",
		}
		ctx.Response(http.StatusOK, resp)
		return nil
	})

	defer ms.Cleanup()
	ms.Start()
}
