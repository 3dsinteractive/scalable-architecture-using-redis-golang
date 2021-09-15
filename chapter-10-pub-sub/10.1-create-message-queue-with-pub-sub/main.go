package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/3dsinteractive/wrkgo"
)

const channelRegister = "channel::register"

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

	// Worker 1
	go startRegisterWorker(ms, cfg, "1")
	// Worker 2
	go startRegisterWorker(ms, cfg, "2")
	// Worker 3
	go startRegisterWorker(ms, cfg, "3")

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

		// Create register payload to publish to subscriber
		username, _ := payload["username"].(string)
		registerPayload := &RegisterPayload{
			TransactionID: NewUUID(),
			Username:      username,
		}
		registerPayloadJS, err := json.Marshal(registerPayload)
		if err != nil {
			ctx.Response(http.StatusOK, map[string]interface{}{
				"status": "invalid input",
				"error":  err.Error(),
			})
			return nil
		}

		cacher := ctx.Cacher(cfg.CacherConfig())
		err = cacher.Pub(channelRegister, string(registerPayloadJS))
		if err != nil {
			ctx.Response(http.StatusOK, map[string]interface{}{
				"status": "error",
				"error":  err.Error(),
			})
			return nil
		}

		// Response transactionID to refer later, if needed
		resp := map[string]interface{}{
			"status":         "ok",
			"transaction_id": registerPayload.TransactionID,
		}
		ctx.Response(http.StatusOK, resp)
		return nil
	})

	// 5. Cleanup when exit
	defer ms.Cleanup()
	ms.Start()
}

func startRegisterWorker(ms *Microservice, cfg IConfig, workerID string) {

	ms.Log("Subscriber", fmt.Sprintf("Worker %s is starting", workerID))

	cacher := ms.Cacher(cfg.CacherConfig())
	onRegister, subID, err := cacher.Sub(channelRegister)
	if err != nil {
		ms.Log("Subscriber", err.Error())
		return
	}

	osQuit := make(chan os.Signal, 1)
	signal.Notify(osQuit, syscall.SIGTERM, syscall.SIGINT)

	// Loop to listen for payload from API
	for {
		select {
		case msg := <-onRegister:
			if msg == nil {
				// This happen when cacher close
				return
			}

			payloadStr := msg.Payload
			payload := &RegisterPayload{}
			err = json.Unmarshal([]byte(payloadStr), &payload)
			if err != nil {
				ms.Log("Subscriber", err.Error())
				continue
			}

			selectedWorker, err := isSelectedWorker(cacher, payload.TransactionID)
			if err != nil {
				ms.Log("Subscriber", err.Error())
				continue
			}
			// If not a selected worker, continue to wait for next payload
			if !selectedWorker {
				continue
			}

			// ms.Log("Subscriber", fmt.Sprintf("Worker %s is selected", workerID))

			duplicated, err := isDuplidatedUsernameInCache(cacher, payload.Username)
			if err != nil {
				ms.Log("Subscriber", err.Error())
				continue
			}

			if duplicated {
				// ms.Log("Subscriber", "duplicated")
				continue
			}

			err = createMemberInCache(cacher, payload.Username)
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
			os.Exit(0)
		}
	}
}

type RegisterPayload struct {
	TransactionID string `json:"transaction_id"`
	Username      string `json:"username"`
}

func isSelectedWorker(cacher ICacher, transactionID string) (bool, error) {
	cacheKey := fmt.Sprintf("transaction::%s", transactionID)
	id, err := cacher.Incr(cacheKey)
	if err != nil {
		return false, err
	}
	// Expire key in 60 seconds after use
	cacher.Expire(cacheKey, 60*time.Second)
	// only first worker that call this function will get id == 1,
	// if the worker get the id == 1, so it is the selected worker
	if id == 1 {
		return true, nil
	}
	return false, nil
}

func isDuplidatedUsernameInCache(cacher ICacher, username string) (bool, error) {
	cacheKey := getRegisterCacheKey(username)
	exists, err := cacher.Exists(cacheKey)
	if err != nil {
		return false, nil
	}
	return exists, nil
}

func createMemberInCache(cacher ICacher, username string) error {
	next, err := nextRegisterOrder(cacher)
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

func nextRegisterOrder(cacher ICacher) (int, error) {
	next, err := cacher.Autonumber("members::autonumber")
	if err != nil {
		return 0, err
	}
	return next, nil
}

func getRegisterCacheKey(username string) string {
	return fmt.Sprintf("register::%s", username)
}
