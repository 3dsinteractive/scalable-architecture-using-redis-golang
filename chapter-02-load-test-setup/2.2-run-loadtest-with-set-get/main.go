package main

import (
	"net/http"

	_ "github.com/3dsinteractive/wrkgo"
)

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

	// 3. GET api query direct from database
	ms.GET("/api", func(ctx IContext) error {
		members, err := queryLastestMembersFromDatabase(ctx, cfg)
		if err != nil {
			ctx.Response(http.StatusInternalServerError, map[string]interface{}{"status": "error"})
			return nil
		}

		resp := map[string]interface{}{
			"status": "ok",
			"items":  members,
		}
		ctx.Response(http.StatusOK, resp)
		return nil
	})

	// 4. GET api using cache at data layer
	//    the benefit of data layer cache, is data can be shared at other api
	// ms.GET("/api", func(ctx IContext) error {

	// 	cacheKey := "members::latest"
	// 	members := []*Member{}

	// 	cacheTimeout := 60 * 2 * time.Second
	// 	cacher := ctx.Cacher(NewCacherConfig())
	// 	membersJS, err := cacher.Get(cacheKey)
	// 	if err != nil {
	// 		ctx.Log(err.Error())
	// 	}

	// 	if len(membersJS) > 0 {
	// 		// ctx.Log("cache hit")
	// 		err := json.Unmarshal([]byte(membersJS), &members)
	// 		if err != nil {
	// 			cacher.Del(cacheKey)
	// 			ctx.Log(err.Error())
	// 		}
	// 	}

	// 	if len(membersJS) == 0 {
	// 		// ctx.Log("cache miss")
	// 		members, err := queryLastestMembersFromDatabase(ctx, cfg)
	// 		if err != nil {
	// 			ctx.Response(http.StatusInternalServerError, map[string]interface{}{"status": "error"})
	// 			return nil
	// 		}

	// 		err = cacher.Set(cacheKey, members, cacheTimeout)
	// 		if err != nil {
	// 			ctx.Log(err.Error())
	// 		}
	// 	}

	// 	resp := map[string]interface{}{
	// 		"status": "ok",
	// 		"items":  members,
	// 	}
	// 	ctx.Response(http.StatusOK, resp)
	// 	return nil
	// })

	// 5. GET api using cache at api layer
	//    the benefit of api layer cache, is the fatest cache
	// ms.GET("/api", func(ctx IContext) error {

	// 	cacheKey := "api::members::latest"
	// 	cacheTimeout := 60 * 2 * time.Second
	// 	cacher := ctx.Cacher(NewCacherConfig())
	// 	apiJS, err := cacher.Get(cacheKey)
	// 	if err != nil {
	// 		ctx.Log(err.Error())
	// 	}

	// 	if len(apiJS) > 0 {
	// 		// ctx.Log("cache hit")
	// 		ctx.ResponseS(http.StatusOK, apiJS)
	// 		return nil
	// 	}

	// 	// ctx.Log("cache miss")
	// 	members, err := queryLastestMembersFromDatabase(ctx, cfg)
	// 	if err != nil {
	// 		ctx.Response(http.StatusInternalServerError, map[string]interface{}{"status": "error"})
	// 		return nil
	// 	}

	// 	resp := map[string]interface{}{
	// 		"status": "ok",
	// 		"items":  members,
	// 	}

	// 	respString, err := json.Marshal(resp)
	// 	if err != nil {
	// 		ctx.Response(http.StatusInternalServerError, map[string]interface{}{"status": "error"})
	// 		return nil
	// 	}

	// 	// Cache api response
	// 	err = cacher.SetS(cacheKey, string(respString), cacheTimeout)
	// 	if err != nil {
	// 		ctx.Log(err.Error())
	// 	}

	// 	ctx.ResponseS(http.StatusOK, string(respString))
	// 	return nil
	// })

	// 5. Cleanup when exit
	defer ms.Cleanup()
	ms.Start()
}

func queryLastestMembersFromDatabase(ctx IContext, cfg IConfig) ([]*Member, error) {
	pst := ctx.Persister(cfg.PersisterConfig())
	members := make([]*Member, 0)
	_, err := pst.WhereSP(
		&members,
		"register_order desc",
		30, // limits
		1,  // pages
		"is_active = ?",
		"1")
	if err != nil {
		return nil, err
	}
	return members, nil
}
