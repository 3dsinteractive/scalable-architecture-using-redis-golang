package main

import (
	"encoding/json"
	"net/http"
	"strconv"

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

	// 3. Register vote api use redis
	ms.POST("/vote", func(ctx IContext) error {
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

		// worldCitizenID has value between 0 - 2,000,000,000
		// each world citizen can vote yes or no
		// so we will use 2,000,000,000 bits
		citizenIDStr, ok := payload["world_citizen_id"].(string)
		if !ok {
			ctx.Response(http.StatusOK, map[string]interface{}{"status": "invalid input"})
			return nil
		}
		cititzenID, err := strconv.Atoi(citizenIDStr)
		if err != nil {
			ctx.Response(http.StatusOK, map[string]interface{}{
				"status": "invalid input",
				"error":  err.Error(),
			})
			return nil
		}

		// vote value (yes/no)
		voteYes := false
		voteStr, ok := payload["vote"].(string)
		if ok && voteStr == "yes" {
			voteYes = true
		}

		_, err = vote(ctx, cfg, cititzenID, voteYes)
		if err != nil {
			ctx.Response(http.StatusInternalServerError, map[string]interface{}{
				"status": "error",
				"error":  err.Error(),
			})
			return nil
		}

		resp := map[string]interface{}{
			"status":     "ok",
			"citizen_id": cititzenID,
			"vote":       voteStr,
		}
		ctx.Response(http.StatusOK, resp)
		return nil
	})

	// 5. Cleanup when exit
	defer ms.Cleanup()
	ms.Start()
}

func vote(ctx IContext, cfg IConfig, citizenID int, voteYes bool) (int /*prev value*/, error) {
	cacher := ctx.Cacher(cfg.CacherConfig())
	cacheKey := "vote"

	voteVal := 0
	if voteYes {
		voteVal = 1
	}
	val, err := cacher.BitFieldSet(cacheKey, 1, citizenID, voteVal)
	return int(val), err
}
