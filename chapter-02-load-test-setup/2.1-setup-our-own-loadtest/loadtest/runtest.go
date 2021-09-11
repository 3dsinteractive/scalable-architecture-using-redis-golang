package main

import (
	"fmt"
	"time"
)

func main() {

	config := &LoadTestConfig{
		BaseURL:         "http://localhost:8080",
		ConcurrentUsers: 50,
		RunDuration:     time.Second * 10,
		DebugError:      true,
		DebugRequest:    false,
		DebugResponse:   false,
	}

	templates := []*LoadTestTemplate{
		{
			ID:      "0",
			URLPath: "/api",
			Timeout: time.Second * 3,
			Method:  "GET",
			Headers: map[string]string{
				"Content-Type": "application/json; charset=UTF-8",
			},
		},
	}

	reqSetupHandler := func(tmpl *LoadTestTemplate, req *LoadTestRequest, prevResp *LoadTestResponse) error {
		return nil
	}

	lt := NewLoadTest()
	err := lt.Run(config, templates, reqSetupHandler)
	if err != nil {
		fmt.Printf(err.Error())
	}
}
