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

	// 2. Migrate and seed 100,000 members
	ms.Log("Main", "Migrate database...")
	err := setup(cfg)
	if err != nil {
		ms.Log("Main", err.Error())
		return
	}

	// 3. Register api use redis
	ms.POST("/register", func(ctx IContext) error {
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

		username, ok := payload["username"].(string)
		if !ok {
			ctx.Response(http.StatusOK, map[string]interface{}{"status": "invalid input"})
			return nil
		}

		duplicated, err := isDuplidatedUsername(ctx, cfg, username)
		if err != nil {
			ctx.Response(http.StatusInternalServerError, map[string]interface{}{
				"status": "error",
				"error":  err.Error(),
			})
			return nil
		}
		if duplicated {
			ctx.Response(http.StatusOK, map[string]interface{}{"status": "duplicated"})
			return nil
		}

		err = createMember(ctx, cfg, username)
		if err != nil {
			ctx.Response(http.StatusInternalServerError, map[string]interface{}{
				"status": "error",
				"error":  err.Error(),
			})
			return nil
		}

		resp := map[string]interface{}{
			"status": "ok",
		}
		ctx.Response(http.StatusOK, resp)
		return nil
	})

	// 4. Register api use custom shardings
	// ms.POST("/register", func(ctx IContext) error {
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

	// 	username, ok := payload["username"].(string)
	// 	if !ok {
	// 		ctx.Response(http.StatusOK, map[string]interface{}{"status": "invalid input"})
	// 		return nil
	// 	}

	// 	duplicated, err := isDuplidatedUsernameInShard(ctx, cfg, username)
	// 	if err != nil {
	// 		ctx.Response(http.StatusInternalServerError, map[string]interface{}{
	// 			"status": "error",
	// 			"error":  err.Error(),
	// 		})
	// 		return nil
	// 	}
	// 	if duplicated {
	// 		ctx.Response(http.StatusOK, map[string]interface{}{"status": "duplicated"})
	// 		return nil
	// 	}

	// 	err = createMemberInShard(ctx, cfg, username)
	// 	if err != nil {
	// 		ctx.Response(http.StatusInternalServerError, map[string]interface{}{
	// 			"status": "error",
	// 			"error":  err.Error(),
	// 		})
	// 		return nil
	// 	}

	// 	resp := map[string]interface{}{
	// 		"status": "ok",
	// 	}
	// 	ctx.Response(http.StatusOK, resp)
	// 	return nil
	// })

	// 5. Cleanup when exit
	defer ms.Cleanup()
	ms.Start()
}

func getConfigOfShards(cfg IConfig, username string) ICacherConfig {
	cfgs := []ICacherConfig{
		cfg.CacherConfig1(),
		cfg.CacherConfig2(),
		cfg.CacherConfig3(),
		cfg.CacherConfig4(),
		cfg.CacherConfig5(),
	}
	hash := FastHash(username)
	shards := hash % uint64(len(cfgs))
	return cfgs[shards]
}

func isDuplidatedUsernameInShard(ctx IContext, cfg IConfig, username string) (bool, error) {
	cacheCfg := getConfigOfShards(cfg, username)
	cacher := ctx.Cacher(cacheCfg)
	cacheKey := getRegisterCacheKey(username)
	exists, err := cacher.Exists(cacheKey)
	if err != nil {
		return false, nil
	}
	return exists, nil
}

func createMemberInShard(ctx IContext, cfg IConfig, username string) error {

	next, err := nextRegisterOrder(ctx, cfg)
	if err != nil {
		return err
	}

	cacheCfg := getConfigOfShards(cfg, username)
	cacher := ctx.Cacher(cacheCfg)
	member := &Member{
		ID:            NewUUID(),
		Username:      username,
		RegisterOrder: next,
		IsActive:      1,
	}

	cacheKey := getRegisterCacheKey(username)
	err = cacher.SetNoExpire(cacheKey, member)
	if err != nil {
		return err
	}

	return nil
}

func isDuplidatedUsername(ctx IContext, cfg IConfig, username string) (bool, error) {
	cacher := ctx.Cacher(cfg.CacherConfig1())
	cacheKey := getRegisterCacheKey(username)
	exists, err := cacher.Exists(cacheKey)
	if err != nil {
		return false, nil
	}
	return exists, nil
}

func createMember(ctx IContext, cfg IConfig, username string) error {

	next, err := nextRegisterOrder(ctx, cfg)
	if err != nil {
		return err
	}

	cacher := ctx.Cacher(cfg.CacherConfig1())
	member := &Member{
		ID:            NewUUID(),
		Username:      username,
		RegisterOrder: next,
		IsActive:      1,
	}

	cacheKey := getRegisterCacheKey(username)
	err = cacher.SetNoExpire(cacheKey, member)
	if err != nil {
		return err
	}

	return nil
}

func getRegisterCacheKey(username string) string {
	return fmt.Sprintf("register::%s", username)
}

func nextRegisterOrder(ctx IContext, cfg IConfig) (int, error) {
	cacher := ctx.Cacher(cfg.CacherConfig1())
	next, err := cacher.Autonumber("members::autonumber")
	if err != nil {
		return 0, err
	}
	return next, nil
}
