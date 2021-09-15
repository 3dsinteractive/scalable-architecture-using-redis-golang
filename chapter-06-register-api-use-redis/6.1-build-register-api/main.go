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

	// 3. Register api  register direct to mysql
	ms.POST("/register", func(ctx IContext) error {
		// input format {"username":"username_01@domain.com"}
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

	// 4. Register api use redis as database
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

	// 	duplicated, err := isDuplidatedUsernameInCache(ctx, cfg, username)
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

	// 	err = createMemberInCache(ctx, cfg, username)
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

	// 5. Register api use buffering
	// buffer := map[string]interface{}{} // map[username] => struct{}{}
	// bufferMutex := sync.Mutex{}

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

	// 	bufferMutex.Lock()
	// 	buffer[username] = struct{}{}
	// 	bufferMutex.Unlock()

	// 	resp := map[string]interface{}{
	// 		"status": "ok",
	// 	}
	// 	ctx.Response(http.StatusOK, resp)
	// 	return nil
	// })

	// go func() {
	// 	t := time.NewTicker(time.Millisecond * 500)
	// 	for range t.C {

	// 		if len(buffer) == 0 {
	// 			continue
	// 		}

	// 		bufferMutex.Lock()

	// 		usernames := []string{}
	// 		for username := range buffer {
	// 			// ms.Log("Worker", fmt.Sprintf("register %s", username))
	// 			usernames = append(usernames, username)
	// 		}

	// 		cacher := ms.Cacher(cfg.CacherConfig())
	// 		exists, err := isDuplidatedUsernamesInBatch(cacher, usernames)
	// 		if err != nil {
	// 			ms.Log("Worker", "error: "+err.Error())
	// 		}

	// 		registers := []string{}
	// 		for i, username := range usernames {
	// 			exist := exists[i]
	// 			if !exist {
	// 				registers = append(registers, username)
	// 			}
	// 		}

	// 		if len(registers) > 0 {
	// 			err := createMemberInBatch(cacher, registers)
	// 			if err != nil {
	// 				ms.Log("Worker", "error: "+err.Error())
	// 			}
	// 		}

	// 		buffer = map[string]interface{}{}
	// 		bufferMutex.Unlock()
	// 	}
	// }()

	// 5. Cleanup when exit
	defer ms.Cleanup()
	ms.Start()
}

func isDuplidatedUsername(ctx IContext, cfg IConfig, username string) (bool, error) {
	pst := ctx.Persister(cfg.PersisterConfig())
	members := make([]*Member, 0)
	_, err := pst.WhereP(
		&members,
		1, // limits
		1, // pages
		"username = ?",
		username)
	if err != nil {
		return false, nil
	}

	return len(members) > 0, nil
}

func createMember(ctx IContext, cfg IConfig, username string) error {

	next, err := nextRegisterOrder(ctx, cfg)
	if err != nil {
		return err
	}

	pst := ctx.Persister(cfg.PersisterConfig())
	member := &Member{
		ID:            NewUUID(),
		Username:      username,
		RegisterOrder: next,
		IsActive:      1,
	}

	err = pst.Create(member)
	if err != nil {
		return err
	}

	return nil
}

func isDuplidatedUsernameInCache(ctx IContext, cfg IConfig, username string) (bool, error) {
	cacher := ctx.Cacher(cfg.CacherConfig())
	cacheKey := getRegisterCacheKey(username)
	exists, err := cacher.Exists(cacheKey)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func createMemberInCache(ctx IContext, cfg IConfig, username string) error {

	next, err := nextRegisterOrder(ctx, cfg)
	if err != nil {
		return err
	}

	cacher := ctx.Cacher(cfg.CacherConfig())
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

func isDuplidatedUsernamesInBatch(cacher ICacher, usernames []string) ([]bool, error) {
	if len(usernames) == 0 {
		return nil, nil
	}

	cacheKeys := make([]string, len(usernames))
	for i, username := range usernames {
		cacheKey := getRegisterCacheKey(username)
		cacheKeys[i] = cacheKey
	}

	cacheItems, err := cacher.MGet(cacheKeys)
	if err != nil {
		return nil, err
	}
	exists := make([]bool, len(cacheItems))
	for i, cacheItem := range cacheItems {
		exists[i] = (cacheItem != nil)
	}
	return exists, nil
}

func createMemberInBatch(cacher ICacher, usernames []string) error {
	if len(usernames) == 0 {
		return nil
	}

	nexts, err := nextRegisterOrderInBatch(cacher, len(usernames))
	if err != nil {
		return err
	}

	members := map[string]interface{}{}
	for i, username := range usernames {
		cacheKey := getRegisterCacheKey(username)
		members[cacheKey] = &Member{
			ID:            NewUUID(),
			Username:      username,
			RegisterOrder: nexts[i],
			IsActive:      1,
		}
	}

	err = cacher.MSet(members)
	if err != nil {
		return err
	}

	return nil
}

func getRegisterCacheKey(username string) string {
	return fmt.Sprintf("register::%s", username)
}

func nextRegisterOrder(ctx IContext, cfg IConfig) (int, error) {
	cacher := ctx.Cacher(cfg.CacherConfig())
	next, err := cacher.Autonumber("members::autonumber")
	if err != nil {
		return 0, err
	}
	return next, nil
}

func nextRegisterOrderInBatch(cacher ICacher, numberOfUsers int) ([]int, error) {
	nexts, err := cacher.Autonumbers("members::autonumber", numberOfUsers)
	if err != nil {
		return nil, err
	}
	return nexts, nil
}
