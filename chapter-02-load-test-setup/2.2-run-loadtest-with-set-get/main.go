package main

import (
	"net/http"
	"time"

	_ "github.com/3dsinteractive/wrkgo"
)

func main() {

	ms := NewMicroservice()

	ms.GET("/api", func(ctx IContext) error {
		titles := queryNameTitlesFromDatabase()
		resp := map[string]interface{}{
			"status": "ok",
			"items":  titles,
		}
		ctx.Response(http.StatusOK, resp)
		return nil
	})

	// ms.GET("/api", func(ctx IContext) error {
	// 	titles := queryNameTitlesFromDatabase()
	// 	resp := map[string]interface{}{
	// 		"status": "ok",
	// 		"items":  titles,
	// 	}
	// 	ctx.Response(http.StatusOK, resp)
	// 	return nil
	// })

	defer ms.Cleanup()
	ms.Start()
}

func queryNameTitlesFromDatabase() []string {
	time.Sleep(50 * time.Millisecond)
	return []string{
		"Dr.",
		"Mr.",
		"Mrs.",
		"Ms.",
		"Prof.",
	}
}
