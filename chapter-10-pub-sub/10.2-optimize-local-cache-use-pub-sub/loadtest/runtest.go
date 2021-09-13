package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/3dsinteractive/wrkgo"
)

func main() {

	config := &wrkgo.LoadTestConfig{
		BaseURL:         "http://localhost:8080",
		ConcurrentUsers: 50,
		RunDuration:     time.Second * 10,
		DebugError:      true,
		DebugRequest:    false,
		DebugResponse:   false,
	}

	templates := []*wrkgo.LoadTestTemplate{
		{
			ID:      "0",
			URLPath: "/level",
			Timeout: time.Second * 3,
			Method:  "GET",
			Headers: map[string]string{
				"Content-Type": "application/json; charset=UTF-8",
			},
		},
	}

	reqSetupHandler := func(tmpl *wrkgo.LoadTestTemplate, req *wrkgo.LoadTestRequest, prevResp *wrkgo.LoadTestResponse) error {
		// Random user name
		username := fmt.Sprintf("user_%d", RandomMinMax(0, 5))
		req.SetURL(config.BaseURL + tmpl.URLPath + "?u=" + username)
		return nil
	}

	lt := wrkgo.NewLoadTest()
	err := lt.Run(config, templates, reqSetupHandler)
	if err != nil {
		fmt.Printf(err.Error())
	}
}

func RandomMinMax(min int, max int) int {
	s := rand.NewSource(time.Now().UnixNano())
	r := rand.New(s)
	return r.Intn(max-min+1) + min
}
