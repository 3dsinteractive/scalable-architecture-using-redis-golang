package main

import (
	"fmt"
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
			URLPath: "/api",
			Timeout: time.Second * 6,
			Method:  "GET",
			Headers: map[string]string{
				"Content-Type": "application/json; charset=UTF-8",
			},
		},
	}

	reqSetupHandler := func(tmpl *wrkgo.LoadTestTemplate, req *wrkgo.LoadTestRequest, prevResp *wrkgo.LoadTestResponse) error {
		return nil
	}

	lt := wrkgo.NewLoadTest()
	err := lt.Run(config, templates, reqSetupHandler)
	if err != nil {
		fmt.Printf(err.Error())
	}
}
