// PAM Engine & Library is proprietary and confidential.
// Un-authorize using, editing, copying, adapting, distributing, of this file or some part of this file without
// the prior written consent of PushAndMotion, via any medium is strictly prohibited.
// If not expressively specify in the document, the authorisation to use this library will be granted per application.
// Any question regarding this copyright notice please contact contact@pushandmotion.com
// This copyright notice must be included in the header of every distribution of all the source code.

package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"os"
	"time"

	"github.com/briandowns/spinner"
	humanize "github.com/dustin/go-humanize"
	"github.com/glentiki/hdrhistogram"
	"github.com/olekukonko/tablewriter"
	"github.com/ttacon/chalk"
	"github.com/valyala/fasthttp"
)

type SetupRequestFunc func(tmpl *LoadTestTemplate, req *LoadTestRequest, prevResp *LoadTestResponse) error

type ILoadTest interface {
	Run(config *LoadTestConfig, templates []*LoadTestTemplate, setupHandler SetupRequestFunc) error
}

type LoadTest struct{}

func NewLoadTest() *LoadTest {
	return &LoadTest{}
}

func (lt *LoadTest) Run(
	config *LoadTestConfig,
	templates []*LoadTestTemplate,
	setupHandler SetupRequestFunc) error {

	respChan, errChan := lt.runClients(config, templates, setupHandler)
	if respChan == nil || errChan == nil {
		fmt.Println("No load test templates!")
		return nil
	}

	latencies := hdrhistogram.New(1, 10000, 5)
	requests := hdrhistogram.New(1, 1000000, 5)
	throughput := hdrhistogram.New(1, 100000000000, 5)

	var bytes int64 = 0
	var totalBytes int64 = 0
	var respCounter int64 = 0
	var totalResp int64 = 0

	resp2xx := 0
	respN2xx := 0

	errors := 0
	timeouts := 0

	ticker := time.NewTicker(time.Second)
	runTimeout := time.NewTimer(config.RunDuration)

	debugError := config.DebugError
	debugResp := config.DebugResponse

	spin := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	spin.Suffix = " Running Loadtest..."
	spin.Start()

	for {
		select {
		case err := <-errChan:
			errors++
			if debugError {
				fmt.Printf("debug error: %s\n", err.Error())
			}
			if err == fasthttp.ErrTimeout {
				timeouts++
			}
		case res := <-respChan:
			s := int64(res.size)
			bytes += s
			totalBytes += s
			respCounter++

			totalResp++
			if res.status >= 200 && res.status < 300 {
				latencies.RecordValue(int64(res.latency))
				if debugResp {
					fmt.Printf("debug 2xx response: %s\n", res.GetBody())
				}
				resp2xx++
			} else {
				if debugError {
					fmt.Printf("debug non 2xx response: %s\n", res.GetBody())
				}
				respN2xx++
			}
		case <-ticker.C:
			requests.RecordValue(respCounter)
			respCounter = 0
			throughput.RecordValue(bytes)
			bytes = 0
		case <-runTimeout.C:
			spin.Stop()

			fmt.Println("")
			fmt.Println("")
			shortLatency := tablewriter.NewWriter(os.Stdout)
			shortLatency.SetRowSeparator("-")
			shortLatency.SetHeader([]string{
				"Stat",
				"50%",
				"97.5%",
				"99%",
				"Avg",
				"Stdev",
				"Max",
			})
			shortLatency.SetHeaderColor(
				tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
				tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
				tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
				tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
				tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
				tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
				tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor})
			shortLatency.Append([]string{
				chalk.Bold.TextStyle("Latency"),
				fmt.Sprintf("%v ms", latencies.ValueAtPercentile(50)),
				fmt.Sprintf("%v ms", latencies.ValueAtPercentile(97.5)),
				fmt.Sprintf("%v ms", latencies.ValueAtPercentile(99)),
				fmt.Sprintf("%.2f ms", latencies.Mean()),
				fmt.Sprintf("%.2f ms", latencies.StdDev()),
				fmt.Sprintf("%v ms", latencies.Max()),
			})
			shortLatency.Render()
			fmt.Println("")
			fmt.Println("")

			requestsTable := tablewriter.NewWriter(os.Stdout)
			requestsTable.SetRowSeparator("-")
			requestsTable.SetHeader([]string{
				"Stat",
				"50%",
				"97.5%",
				"99%",
				"Avg",
				"Stdev",
				"Min",
			})
			requestsTable.SetHeaderColor(
				tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
				tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
				tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
				tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
				tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
				tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
				tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor})
			requestsTable.Append([]string{
				chalk.Bold.TextStyle("Req/Sec"),
				fmt.Sprintf("%v", requests.ValueAtPercentile(50)),
				fmt.Sprintf("%v", requests.ValueAtPercentile(97.5)),
				fmt.Sprintf("%v", requests.ValueAtPercentile(99)),
				fmt.Sprintf("%.2f", requests.Mean()),
				fmt.Sprintf("%.2f", requests.StdDev()),
				fmt.Sprintf("%v", requests.Min()),
			})

			requestsTable.Append([]string{
				chalk.Bold.TextStyle("Bytes/Sec"),
				fmt.Sprintf("%v", humanize.Bytes(uint64(throughput.ValueAtPercentile(50)))),
				fmt.Sprintf("%v", humanize.Bytes(uint64(throughput.ValueAtPercentile(97.5)))),
				fmt.Sprintf("%v", humanize.Bytes(uint64(throughput.ValueAtPercentile(99)))),
				fmt.Sprintf("%v", humanize.Bytes(uint64(throughput.Mean()))),
				fmt.Sprintf("%v", humanize.Bytes(uint64(throughput.StdDev()))),
				fmt.Sprintf("%v", humanize.Bytes(uint64(throughput.Min()))),
			})
			requestsTable.Render()

			fmt.Println("")
			fmt.Println("Req/Bytes counts sampled once per second.")
			fmt.Println("")
			fmt.Println("")
			fmt.Printf("%v 2xx responses, %v non 2xx responses.\n", resp2xx, respN2xx)
			fmt.Printf("%v total requests in %v seconds, %s read.\n", lt.formatBigNum(float64(totalResp)), config.RunDuration, humanize.Bytes(uint64(totalBytes)))
			if errors > 0 {
				fmt.Printf("%v total errors (%v timeouts).\n", lt.formatBigNum(float64(errors)), lt.formatBigNum(float64(timeouts)))
			}
			fmt.Println("Done!")

			return nil
		}
	}
}

func (lt *LoadTest) runClients(
	config *LoadTestConfig,
	templates []*LoadTestTemplate,
	setupHandler SetupRequestFunc) (<-chan *LoadTestResponse, <-chan error) {

	if len(templates) == 0 {
		return nil, nil
	}

	users := config.ConcurrentUsers
	pipeliningFactor := 1
	respChan := make(chan *LoadTestResponse, 2*users*pipeliningFactor)
	errChan := make(chan error, 2*users*pipeliningFactor)

	uri := config.BaseURL
	u, _ := url.Parse(uri)

	for i := 0; i < users; i++ {

		// default port if 440 or 80 if not specify
		port := u.Port()
		if len(port) == 0 {
			if u.Scheme == "https" {
				port = "443"
			} else {
				port = "80"
			}
		}

		c := fasthttp.PipelineClient{
			Addr:               fmt.Sprintf("%v:%v", u.Hostname(), port),
			IsTLS:              u.Scheme == "https",
			MaxPendingRequests: pipeliningFactor,
		}

		for j := 0; j < pipeliningFactor; j++ {
			go func() {

				req := &LoadTestRequest{
					req: fasthttp.AcquireRequest(),
				}

				res := fasthttp.AcquireResponse()

				templateSize := len(templates)
				templatePos := -1

				var prevResp *LoadTestResponse

				for {

					templatePos++
					if templatePos >= templateSize {
						templatePos = 0
						prevResp = nil
					}

					curTemplate := templates[templatePos]
					templateURI := config.BaseURL + curTemplate.URLPath

					req.Reset()
					res.Reset()

					req.SetURL(templateURI)
					req.SetHeaders(curTemplate.Headers)
					req.SetMethod(curTemplate.GetMethod())

					// setup request handler by callback, the prevResp can be nil if there is error from prev request
					err := setupHandler(curTemplate, req, prevResp)
					if err != nil {
						// If setup return error, we reset to template -1 and continue
						templatePos = -1
						prevResp = nil
						continue
					}

					startTime := time.Now()

					if config.DebugRequest {
						fmt.Printf("debug request: method=%s url=%s body=%s\n", req.GetMethod(), req.GetURL(), req.GetBody())
					}

					err = c.DoTimeout(req.req, res, curTemplate.GetTimeout())
					if err != nil {
						prevResp = nil
						errChan <- err
						continue
					}

					body := res.Body()
					bodyStr := string(body)
					size := len(bodyStr) + 2
					res.Header.VisitAll(func(key, value []byte) {
						size += len(key) + len(value) + 2
					})
					prevResp = &LoadTestResponse{
						status:  res.Header.StatusCode(),
						latency: time.Since(startTime).Milliseconds(),
						size:    size,
						body:    bodyStr,
					}
					respChan <- prevResp
				}
			}()
		}
	}

	return respChan, errChan
}

func (lt *LoadTest) formatBigNum(i float64) string {
	if i < 1000 {
		return fmt.Sprintf("%.0f", i)
	}
	return fmt.Sprintf("%.0fk", math.Round(i/1000))
}

type LoadTestConfig struct {
	BaseURL         string        `json:"base_url"`
	ConcurrentUsers int           `json:"concurrent_users"`
	RunDuration     time.Duration `json:"run_duration"`
	DebugError      bool          `json:"debug_error"`
	DebugRequest    bool          `json:"debug_request"`
	DebugResponse   bool          `json:"debug_response"`
}

type LoadTestTemplate struct {
	ID      string            `json:"id"`
	URLPath string            `json:"url_path"`
	Method  string            `json:"method"`
	Timeout time.Duration     `json:"timeout"`
	Headers map[string]string `json:"headers"`
}

// GetMethod return http method, default is GET
func (tmpl *LoadTestTemplate) GetMethod() string {
	// Default is GET
	if len(tmpl.Method) == 0 {
		return "GET"
	}
	return tmpl.Method
}

func (tmpl *LoadTestTemplate) GetTimeout() time.Duration {
	// Default timeout is 3 seconds, if not specify
	if tmpl.Timeout == 0 {
		return 3 * time.Second
	}
	return tmpl.Timeout
}

type LoadTestRequest struct {
	req *fasthttp.Request
}

func (req *LoadTestRequest) SetBody(body []byte) {
	req.req.SetBody(body)
}

func (req *LoadTestRequest) GetBody() string {
	return string(req.req.Body())
}

func (req *LoadTestRequest) SetBodyJSON(params map[string]interface{}) {
	js, _ := json.Marshal(params)
	req.SetBody(js)
}

func (req *LoadTestRequest) SetURL(url string) {
	req.req.SetRequestURI(url)
}

func (req *LoadTestRequest) GetURL() string {
	return req.req.URI().String()
}

func (req *LoadTestRequest) SetMethod(method string) {
	req.req.Header.SetMethod(method)
}

func (req *LoadTestRequest) GetMethod() string {
	return string(req.req.Header.Method())
}

func (req *LoadTestRequest) SetHeaders(headers map[string]string) {
	for key, val := range headers {
		req.req.Header.Add(key, val)
	}
}

func (req *LoadTestRequest) Reset() {
	req.req.Reset()
}

type LoadTestResponse struct {
	body    string
	status  int
	latency int64
	size    int
}

func (res *LoadTestResponse) GetBody() string {
	return res.body
}
