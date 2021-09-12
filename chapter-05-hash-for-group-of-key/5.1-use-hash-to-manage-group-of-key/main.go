package main

import (
	"fmt"
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

	// 3. GET points api query direct from database
	ms.GET("/points", func(ctx IContext) error {

		username := ctx.QueryParam("u")
		// Get member points
		points, err := queryMemberPoints(ctx, cfg, username)
		if err != nil {
			ctx.Response(http.StatusInternalServerError, map[string]interface{}{"status": "error"})
			return nil
		}

		resp := map[string]interface{}{
			"status": "ok",
			"points": points,
		}
		ctx.Response(http.StatusOK, resp)
		return nil
	})

	// 4. GET level api query direct from database
	ms.GET("/level", func(ctx IContext) error {

		username := ctx.QueryParam("u")
		// Get member points
		level, err := queryMemberLevel(ctx, cfg, username)
		if err != nil {
			ctx.Response(http.StatusInternalServerError, map[string]interface{}{"status": "error"})
			return nil
		}

		resp := map[string]interface{}{
			"status": "ok",
			"level":  level,
		}
		ctx.Response(http.StatusOK, resp)
		return nil
	})

	// 5. GET points api using HSET, HGET
	// ms.GET("/points", func(ctx IContext) error {

	// 	username := ctx.QueryParam("u")
	// 	cacheKey := getCacheKeyForMember(username)
	// 	cacheField := "points"
	// 	cacheTimeout := 300 * time.Second
	// 	points := -1

	// 	cacher := ctx.Cacher(cfg.CacherConfig())
	// 	pointsJS, err := cacher.HGet(cacheKey, cacheField)
	// 	if err != nil {
	// 		ctx.Log(err.Error())
	// 	}

	// 	if len(pointsJS) > 0 {
	// 		// ctx.Log("cache hit")
	// 		points, err = strconv.Atoi(pointsJS)
	// 		if err != nil {
	// 			points = -1
	// 			cacher.HDel(cacheKey, cacheField)
	// 			ctx.Log(err.Error())
	// 		}
	// 	}

	// 	if points < 0 {
	// 		// ctx.Log("cache miss")
	// 		points, err = queryMemberPoints(ctx, cfg, username)
	// 		if err != nil {
	// 			ctx.Response(http.StatusInternalServerError, map[string]interface{}{"status": "error"})
	// 			return nil
	// 		}
	// 		err = cacher.HSetS(cacheKey, cacheField, fmt.Sprintf("%d", points), cacheTimeout)
	// 		if err != nil {
	// 			ctx.Log(err.Error())
	// 		}
	// 	}

	// 	resp := map[string]interface{}{
	// 		"status": "ok",
	// 		"points": points,
	// 	}
	// 	ctx.Response(http.StatusOK, resp)

	// 	return nil
	// })

	// 6. GET level api using cache
	// ms.GET("/level", func(ctx IContext) error {

	// 	username := ctx.QueryParam("u")
	// 	cacheKey := getCacheKeyForMember(username)
	// 	cacheField := "level"
	// 	cacheTimeout := 300 * time.Second
	// 	level := -1

	// 	cacher := ctx.Cacher(cfg.CacherConfig())
	// 	levelJS, err := cacher.HGet(cacheKey, cacheField)
	// 	if err != nil {
	// 		ctx.Log(err.Error())
	// 	}

	// 	if len(levelJS) > 0 {
	// 		// ctx.Log("cache hit")
	// 		level, err = strconv.Atoi(levelJS)
	// 		if err != nil {
	// 			level = -1
	// 			cacher.HDel(cacheKey, cacheField)
	// 			ctx.Log(err.Error())
	// 		}
	// 	}

	// 	if level < 0 {
	// 		// ctx.Log("cache miss")
	// 		level, err = queryMemberLevel(ctx, cfg, username)
	// 		if err != nil {
	// 			ctx.Response(http.StatusInternalServerError, map[string]interface{}{"status": "error"})
	// 			return nil
	// 		}
	// 		err = cacher.HSetS(cacheKey, cacheField, fmt.Sprintf("%d", level), cacheTimeout)
	// 		if err != nil {
	// 			ctx.Log(err.Error())
	// 		}
	// 	}

	// 	resp := map[string]interface{}{
	// 		"status": "ok",
	// 		"level":  level,
	// 	}
	// 	ctx.Response(http.StatusOK, resp)

	// 	return nil
	// })

	// API to delete member
	ms.DELETE("/member", func(ctx IContext) error {
		username := ctx.QueryParam("u")
		deleteMember(ctx, cfg, username)
		return nil
	})

	// 5. Cleanup when exit
	defer ms.Cleanup()
	ms.Start()
}

func queryMemberLevel(ctx IContext, cfg IConfig, username string) (int /*level*/, error) {
	pst := ctx.Persister(cfg.PersisterConfig())
	members := make([]*Member, 0)
	_, err := pst.WhereP(
		&members,
		1, // limits
		1, // pages
		"username = ?",
		username)
	if err != nil {
		return -1, err
	}

	if len(members) == 0 {
		return -1, fmt.Errorf("member not found")
	}
	return members[0].MemberLevel, nil
}

func queryMemberPoints(ctx IContext, cfg IConfig, username string) (int /*point*/, error) {
	pst := ctx.Persister(cfg.PersisterConfig())
	points := make([]*MemberPoint, 0)
	_, err := pst.WhereP(
		&points,
		1, // limits
		1, // pages
		"username = ?",
		username)
	if err != nil {
		return -1, err
	}

	if len(points) == 0 {
		return -1, fmt.Errorf("member not found")
	}
	return points[0].Point, nil
}

func deleteMember(ctx IContext, cfg IConfig, username string) error {
	pst := ctx.Persister(cfg.PersisterConfig())
	members := make([]*Member, 0)
	_, err := pst.WhereP(
		&members,
		1, // limits
		1, // pages
		"username = ?",
		username)
	if err != nil {
		return err
	}

	if len(members) == 0 {
		return fmt.Errorf("member not found")
	}

	member := members[0]
	member.IsActive = 0

	err = pst.Update(member)
	if err != nil {
		return err
	}

	cacher := ctx.Cacher(cfg.CacherConfig())
	cacheKey := getCacheKeyForMember(username)
	err = cacher.HDel(cacheKey)
	if err != nil {
		return err
	}

	return nil
}

func getCacheKeyForMember(username string) string {
	return fmt.Sprintf("user::%s", username)
}
