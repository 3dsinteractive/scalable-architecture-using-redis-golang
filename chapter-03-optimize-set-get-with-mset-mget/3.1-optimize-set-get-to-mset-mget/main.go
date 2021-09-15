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
		// Query 1
		members, err := queryLastestMembersFromDatabase(ctx, cfg)
		if err != nil {
			ctx.Response(http.StatusInternalServerError, map[string]interface{}{"status": "error"})
			return nil
		}
		// Query 2
		counter, err := queryCountAllMembersFromDatabase(ctx, cfg)
		if err != nil {
			ctx.Response(http.StatusInternalServerError, map[string]interface{}{"status": "error"})
			return nil
		}

		resp := map[string]interface{}{
			"status": "ok",
			"total":  counter,
			"items":  members,
		}
		ctx.Response(http.StatusOK, resp)
		return nil
	})

	// 4. GET api using cache at data layer
	//    the benefit of data layer cache, is data can be shared at other api
	// ms.GET("/api", func(ctx IContext) error {

	// 	query1CacheKey := "members::latest"
	// 	members := []*Member{}

	// 	timeToExpire := 60 * 5 * time.Second // 5m
	// 	cacher := ctx.Cacher(NewCacherConfig())
	// 	membersJS, err := cacher.Get(query1CacheKey)
	// 	if err != nil {
	// 		ctx.Log(err.Error())
	// 	}

	// 	// Found query #1 cache
	// 	if len(membersJS) > 0 {
	// 		// ctx.Log("cache hit")
	// 		err := json.Unmarshal([]byte(membersJS), &members)
	// 		if err != nil {
	// 			cacher.Del(query1CacheKey)
	// 			ctx.Log(err.Error())
	// 		}
	// 	}

	// 	if len(membersJS) == 0 {
	// 		// ctx.Log("cache miss")
	// 		members, err = queryLastestMembersFromDatabase(ctx, cfg)
	// 		if err != nil {
	// 			ctx.Response(http.StatusInternalServerError, map[string]interface{}{"status": "error"})
	// 			return nil
	// 		}

	// 		err = cacher.Set(query1CacheKey, members, timeToExpire)
	// 		if err != nil {
	// 			ctx.Log(err.Error())
	// 		}
	// 	}

	// 	query2CacheKey := "members::total"
	// 	counter := -1
	// 	counterJS, err := cacher.Get(query2CacheKey)
	// 	if err != nil {
	// 		ctx.Log(err.Error())
	// 	}

	// 	// Found query #2 cache
	// 	if len(counterJS) > 0 {
	// 		// ctx.Log("cache hit")
	// 		counter, err = strconv.Atoi(counterJS)
	// 		if err != nil {
	// 			counter = -1
	// 			cacher.Del(query2CacheKey)
	// 			ctx.Log(err.Error())
	// 		}
	// 	}

	// 	if counter < 0 {
	// 		// ctx.Log("cache miss")
	// 		counter, err = queryCountAllMembersFromDatabase(ctx, cfg)
	// 		if err != nil {
	// 			ctx.Response(http.StatusInternalServerError, map[string]interface{}{"status": "error"})
	// 			return nil
	// 		}
	// 		err = cacher.SetS(query2CacheKey, fmt.Sprintf("%d", counter), timeToExpire)
	// 		if err != nil {
	// 			ctx.Log(err.Error())
	// 		}
	// 	}

	// 	resp := map[string]interface{}{
	// 		"status": "ok",
	// 		"total":  counter,
	// 		"items":  members,
	// 	}
	// 	ctx.Response(http.StatusOK, resp)
	// 	return nil
	// })

	// 5. GET api using cache at data layer
	//    using MSET and MGET to optimize
	// ms.GET("/api", func(ctx IContext) error {

	// 	query1CacheKey := "members::latest"
	// 	query2CacheKey := "members::total"

	// 	members := []*Member{}
	// 	counter := -1

	// 	cacher := ctx.Cacher(NewCacherConfig())
	// 	cacheItems, err := cacher.MGet([]string{query1CacheKey, query2CacheKey})
	// 	if err != nil {
	// 		ctx.Log(err.Error())
	// 	}

	// 	// The len of cacheItems will equal to the keys we send when call MGet
	// 	membersJS := cacheItems[0]
	// 	counterJS := cacheItems[1]

	// 	// Found query #1 cache
	// 	if membersJS != nil && len(membersJS.(string)) > 0 {
	// 		// ctx.Log("cache hit")
	// 		err := json.Unmarshal([]byte(membersJS.(string)), &members)
	// 		if err != nil {
	// 			cacher.Del(query1CacheKey)
	// 			ctx.Log(err.Error())
	// 		}
	// 	}

	// 	itemToCaches := map[string]interface{}{}

	// 	if membersJS == nil {
	// 		// ctx.Log("cache miss")
	// 		members, err = queryLastestMembersFromDatabase(ctx, cfg)
	// 		if err != nil {
	// 			ctx.Response(http.StatusInternalServerError, map[string]interface{}{"status": "error"})
	// 			return nil
	// 		}

	// 		itemToCaches[query1CacheKey] = members
	// 	}

	// 	// Found query #2 cache
	// 	if counterJS != nil && len(counterJS.(string)) > 0 {
	// 		// ctx.Log("cache hit")
	// 		counter, err = strconv.Atoi(counterJS.(string))
	// 		if err != nil {
	// 			counter = -1
	// 			cacher.Del(query2CacheKey)
	// 			ctx.Log(err.Error())
	// 		}
	// 	}

	// 	if counter < 0 {
	// 		// ctx.Log("cache miss")
	// 		counter, err = queryCountAllMembersFromDatabase(ctx, cfg)
	// 		if err != nil {
	// 			ctx.Response(http.StatusInternalServerError, map[string]interface{}{"status": "error"})
	// 			return nil
	// 		}

	// 		itemToCaches[query2CacheKey] = fmt.Sprintf("%d", counter)
	// 	}

	// 	if len(itemToCaches) > 0 {
	// 		timeToExpire := 60 * 5 * time.Second // 5m

	// 		// Set cache using MSET
	// 		err = cacher.MSet(itemToCaches)
	// 		if err != nil {
	// 			ctx.Log(err.Error())
	// 		}

	// 		// Set time to expire
	// 		keys := []string{}
	// 		for k := range itemToCaches {
	// 			keys = append(keys, k)
	// 		}
	// 		err = cacher.Expires(keys, timeToExpire)
	// 		if err != nil {
	// 			ctx.Log(err.Error())
	// 		}
	// 	}

	// 	resp := map[string]interface{}{
	// 		"status": "ok",
	// 		"total":  counter,
	// 		"items":  members,
	// 	}
	// 	ctx.Response(http.StatusOK, resp)
	// 	return nil
	// })

	// 6. GET api using cache at data layer and local memcache
	// ms.GET("/api", func(ctx IContext) error {

	// 	query1CacheKey := "members::latest"
	// 	query2CacheKey := "members::total"

	// 	members := []*Member{}
	// 	counter := -1

	// 	// 1. Find from memory first, if found, then return
	// 	keys := []string{query1CacheKey, query2CacheKey}
	// 	memcacher := ctx.MemCacher()
	// 	cacheItems, err := memcacher.MGet(keys)
	// 	if err != nil {
	// 		ctx.Log(err.Error())
	// 	}

	// 	members, _ = cacheItems[0].([]*Member)
	// 	cachedCounter := cacheItems[1]
	// 	if members != nil && cachedCounter != nil {
	// 		counter, _ := cachedCounter.(int)
	// 		resp := map[string]interface{}{
	// 			"status": "ok",
	// 			"total":  counter,
	// 			"items":  members,
	// 		}
	// 		ctx.Response(http.StatusOK, resp)
	// 		return nil
	// 	}

	// 	// 2. Find from redis
	// 	cacher := ctx.Cacher(NewCacherConfig())
	// 	cacheItems, err = cacher.MGet(keys)
	// 	if err != nil {
	// 		ctx.Log(err.Error())
	// 	}

	// 	// The len of cacheItems will equal to the keys we send when call MGet
	// 	membersJS := cacheItems[0]
	// 	counterJS := cacheItems[1]

	// 	localItemToCaches := map[string]interface{}{}
	// 	remoteItemToCaches := map[string]interface{}{}
	// 	// Found query #1 cache
	// 	if membersJS != nil && len(membersJS.(string)) > 0 {
	// 		// ctx.Log("cache hit")
	// 		err := json.Unmarshal([]byte(membersJS.(string)), &members)
	// 		if err != nil {
	// 			cacher.Del(query1CacheKey)
	// 			ctx.Log(err.Error())
	// 		}
	// 		localItemToCaches[query1CacheKey] = members
	// 	}

	// 	if membersJS == nil {
	// 		// ctx.Log("cache miss")
	// 		members, err = queryLastestMembersFromDatabase(ctx, cfg)
	// 		if err != nil {
	// 			ctx.Response(http.StatusInternalServerError, map[string]interface{}{"status": "error"})
	// 			return nil
	// 		}
	// 		localItemToCaches[query1CacheKey] = members
	// 		remoteItemToCaches[query1CacheKey] = members
	// 	}

	// 	// Found query #2 cache
	// 	if counterJS != nil && len(counterJS.(string)) > 0 {
	// 		// ctx.Log("cache hit")
	// 		counter, err = strconv.Atoi(counterJS.(string))
	// 		if err != nil {
	// 			counter = -1
	// 			cacher.Del(query2CacheKey)
	// 			ctx.Log(err.Error())
	// 		}

	// 		localItemToCaches[query2CacheKey] = counter
	// 	}

	// 	if counter < 0 {
	// 		// ctx.Log("cache miss")
	// 		counter, err = queryCountAllMembersFromDatabase(ctx, cfg)
	// 		if err != nil {
	// 			ctx.Response(http.StatusInternalServerError, map[string]interface{}{"status": "error"})
	// 			return nil
	// 		}
	// 		localItemToCaches[query2CacheKey] = counter
	// 		remoteItemToCaches[query2CacheKey] = fmt.Sprintf("%d", counter)
	// 	}

	// 	// If found item to cache locally, cache it
	// 	if len(localItemToCaches) > 0 {

	// 		timeToExpire := 60 * 5 * time.Second // 5m
	// 		// Set into local memory cache
	// 		err = memcacher.MSet(localItemToCaches, timeToExpire)
	// 		if err != nil {
	// 			ctx.Log(err.Error())
	// 		}
	// 	}

	// 	// If found item to cache remotely, cache it
	// 	if len(remoteItemToCaches) > 0 {

	// 		timeToExpire := 60 * 10 * time.Second // 10m

	// 		// Set cache using MSET
	// 		err = cacher.MSet(remoteItemToCaches)
	// 		if err != nil {
	// 			ctx.Log(err.Error())
	// 		}

	// 		// Set time to expire
	// 		keys := []string{}
	// 		for k := range remoteItemToCaches {
	// 			keys = append(keys, k)
	// 		}
	// 		err = cacher.Expires(keys, timeToExpire)
	// 		if err != nil {
	// 			ctx.Log(err.Error())
	// 		}
	// 	}

	// 	resp := map[string]interface{}{
	// 		"status": "ok",
	// 		"total":  counter,
	// 		"items":  members,
	// 	}
	// 	ctx.Response(http.StatusOK, resp)
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

func queryCountAllMembersFromDatabase(ctx IContext, cfg IConfig) (int, error) {
	pst := ctx.Persister(cfg.PersisterConfig())
	member := &Member{}
	counter, err := pst.Count(
		member,
		"is_active = ?",
		"1")
	if err != nil {
		return 0, err
	}
	return int(counter), nil
}
