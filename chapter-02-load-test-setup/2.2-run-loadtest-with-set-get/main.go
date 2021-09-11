package main

import (
	"net/http"

	_ "github.com/3dsinteractive/wrkgo"
)

func main() {

	ms := NewMicroservice()

	ms.GET("/api", func(ctx IContext) error {

		resp := map[string]string{
			"status": "ok",
		}
		ctx.Response(http.StatusOK, resp)
		return nil
	})

	defer ms.Cleanup()
	ms.Start()
}
