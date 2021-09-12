package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	_ "github.com/3dsinteractive/wrkgo"
)

func main() {

	cfg := NewConfig()

	// 1. Create microservices
	ms := NewMicroservice()

	// 2. Setup project
	err := setup(cfg)
	if err != nil {
		ms.Log("Main", err.Error())
		return
	}

	channelRegister := "channel::register"

	go func() {
		cacher := NewCacher(cfg.CacherConfig())
		onRegister, subID, err := cacher.Sub(channelRegister)
		if err != nil {
			ms.Log("Main", err.Error())
			return
		}

		osQuit := make(chan os.Signal, 1)
		signal.Notify(osQuit, syscall.SIGTERM, syscall.SIGINT)

		for {
			select {
			case msg := <-onRegister:
				if msg == nil {
					// This happen when cacher close
					return
				}

				username := msg.Payload

				duplicated, err := isDuplidatedUsernameInCache(cfg, username)
				if err != nil {
					ms.Log("Subscriber", err.Error())
					continue
				}

				if duplicated {
					ms.Log("Subscriber", "duplicated")
					continue
				}

				err = createMemberInCache(cfg, username)
				if err != nil {
					ms.Log("Subscriber", err.Error())
					continue
				}

			case <-osQuit:
				// Unsub from channel
				err = cacher.Unsub(subID)
				if err != nil {
					ms.Log("Subscriber", err.Error())
				}
				// Exit go routine
				cacher.Close()
				os.Exit(0)
			}
		}
	}()

	// 4. Register api use redis
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

		username, _ := payload["username"].(string)
		cacher := ctx.Cacher(cfg.CacherConfig())
		err = cacher.Pub(channelRegister, username)
		if err != nil {
			ctx.Response(http.StatusOK, map[string]interface{}{
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

	// 5. Cleanup when exit
	defer ms.Cleanup()
	ms.Start()
}

var cacherWorker ICacher
var cacherWorkerMutex sync.Mutex

func getCacher(cfg IConfig) ICacher {
	cacher := cacherWorker
	if cacher == nil {
		cacherWorkerMutex.Lock()
		if cacherWorker == nil {
			cacherWorker = NewCacher(cfg.CacherConfig())
		}
		cacherWorkerMutex.Unlock()
		cacher = cacherWorker
	}
	return cacher
}

func isDuplidatedUsernameInCache(cfg IConfig, username string) (bool, error) {

	cacher := getCacher(cfg)
	cacheKey := getRegisterCacheKey(username)
	exists, err := cacher.Exists(cacheKey)
	if err != nil {
		return false, nil
	}
	return exists, nil
}

func createMemberInCache(cfg IConfig, username string) error {

	cacher := getCacher(cfg)

	next, err := nextRegisterOrder(cfg)
	if err != nil {
		return err
	}

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

func nextRegisterOrder(cfg IConfig) (int, error) {

	cacher := getCacher(cfg)

	next, err := cacher.Autonumber("members::autonumber")
	if err != nil {
		return 0, err
	}
	return next, nil
}
