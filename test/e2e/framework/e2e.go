package framework

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/digitalocean/godo"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
	"golang.org/x/oauth2"
)

var (
	testCtx *TestContext
	mu      sync.Mutex
)

type Test struct {
	Name     string        `json:"name"`
	Failed   bool          `json:"failed"`
	Skipped  bool          `json:"skipped"`
	Running  bool          `json:"running"`
	Duration time.Duration `json:"duration"`
}

func (t *Test) TimedOut() bool {
	return t.Duration > 10*time.Minute
}

type Result struct {
	ID    string    `json:"id"`
	RanAt time.Time `json:"ran_at"`

	Tests map[string]*Test `json:"tests"`

	Logs []byte `json:"logs"`
}

func (r *Result) TimedOut() bool {
	return r.Duration() > 30*time.Minute
}

func (r *Result) Duration() time.Duration {
	var duration time.Duration
	for _, t := range r.Tests {
		duration += t.Duration
	}
	return duration
}

func (r *Result) Running() bool {
	for _, t := range r.Tests {
		if t.Running {
			return true
		}
	}
	return false
}

func (r *Result) Failed() bool {
	for _, t := range r.Tests {
		if t.Failed {
			return true
		}
	}
	return false
}

type TestContext struct {
	result *Result

	Seed int64
	Rand *rand.Rand

	Client *godo.Client

	requestDuration *prometheus.HistogramVec
	pusher          *push.Pusher

	e2eServerAddr string
}

func newTestContext(ctx context.Context) (*TestContext, error) {
	testCtx := &TestContext{
		result: &Result{
			ID:    uuid.New().String(),
			RanAt: time.Now(),
			Tests: make(map[string]*Test),
		},
		Seed: time.Now().Unix(),
	}

	id := os.Getenv("TEST_ID")
	if id != "" {
		testCtx.result.ID = id
	}

	seed := os.Getenv("SEED")
	if seed != "" {
		seedInt, err := strconv.ParseInt(seed, 10, 64)
		if err != nil {
			return nil, errors.New("invalid test seed")
		}
		testCtx.Seed = seedInt
	}
	testCtx.Rand = rand.New(rand.NewSource(testCtx.Seed))

	pushAddr := os.Getenv("PUSHGATEWAY_ADDR")
	if pushAddr != "" {
		testCtx.requestDuration = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "godo",
				Subsystem: "e2e",
				Name:      "request_duration_s",
				Help:      "Amount of time api requests take.",
				Buckets: []float64{
					0.001, 0.01,
					0.05, 0.1, 0.15, 0.2, 0.25, 0.3, 0.35, 0.4, 0.45, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0,
					1.25, 1.5, 1.75, 2.0, 2.5, 3.0, 3.5, 4.0, 4.5, 5,
					6, 7, 8, 9, 10, 15, 20, 25, 30, 40, 50, 60,
				},
			},
			[]string{"method", "endpoint", "status", "error"},
		)

		err := prometheus.Register(testCtx.requestDuration)
		if err != nil {
			return nil, errors.New("failed to register request duration metric")
		}
		testCtx.pusher = push.New(pushAddr, "godo-e2e").Gatherer(prometheus.DefaultGatherer)
	}

	accessToken := os.Getenv("ACCESS_TOKEN")
	if accessToken == "" {
		return nil, errors.New("ACCESS_TOKEN must be configured to run E2E tests")
	}
	oauthClient := oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: accessToken,
	}))
	if testCtx.requestDuration != nil {
		oauthClient.Transport = testCtx.roundTrip(oauthClient.Transport)
	}
	testCtx.Client = godo.NewClient(oauthClient)

	testCtx.e2eServerAddr = os.Getenv("E2E_SERVER_ADDR")

	return testCtx, nil
}

func SetupContext(ctx context.Context, t *testing.T) (*TestContext, func(context.Context, *testing.T)) {
	startTime := time.Now()

	mu.Lock()
	defer mu.Unlock()
	if testCtx == nil {
		var err error
		testCtx, err = newTestContext(ctx)
		if err != nil {
			t.Fatalf("failed to initialize test context: %s", err)
		}
	}

	test, ok := testCtx.result.Tests[t.Name()]
	if !ok {
		test = &Test{
			Name:    t.Name(),
			Running: true,
		}
	}
	testCtx.result.Tests[t.Name()] = test

	if testCtx.e2eServerAddr != "" {
		err := testCtx.postResult(ctx)
		if err != nil {
			t.Logf("failed to post result : %s", err)
		}
	}

	return testCtx, func(ctx context.Context, t *testing.T) {
		if testCtx.e2eServerAddr != "" {
			test.Duration = time.Now().Sub(startTime)
			test.Running = false
			test.Failed = t.Failed()
			test.Skipped = t.Skipped()

			err := testCtx.postResult(ctx)
			if err != nil {
				t.Logf("failed to post result : %s", err)
			}
		}

		if testCtx.pusher != nil {
			defer func() {
				err := testCtx.pusher.Push()
				if err != nil {
					t.Logf("failed to push metrics: %s", err)
				}
				t.Log("pushed metrics")
			}()
		}
	}
}

func (c *TestContext) Cleanup(ctx context.Context, t *testing.T, r interface{}) {
	var (
		resourceID   interface{}
		resourceType string
		cleanup      func() (*godo.Response, error)
	)
	switch resource := r.(type) {
	case *godo.Droplet:
		resourceID = resource.ID
		resourceType = "droplet"
		cleanup = func() (*godo.Response, error) { return c.Client.Droplets.Delete(ctx, resource.ID) }
	case *godo.KubernetesCluster:
		resourceID = resource.ID
		resourceType = "kubernetes_cluster"
		cleanup = func() (*godo.Response, error) { return c.Client.Kubernetes.Delete(ctx, resource.ID) }
	case *godo.Key:
		resourceID = resource.ID
		resourceType = "ssh_key"
		cleanup = func() (*godo.Response, error) { return c.Client.Keys.DeleteByID(ctx, resource.ID) }
	case *godo.Tag:
		resourceID = resource.Name
		resourceType = "tag"
		cleanup = func() (*godo.Response, error) { return c.Client.Tags.Delete(ctx, resource.Name) }
	case *godo.LoadBalancer:
		resourceID = resource.ID
		resourceType = "load_balancer"
		cleanup = func() (*godo.Response, error) { return c.Client.LoadBalancers.Delete(ctx, resource.ID) }
	}
	t.Logf("attempting to cleanup %s (%v)", resourceType, resourceID)
	res, err := cleanup()
	if err != nil && res.StatusCode != http.StatusNotFound {
		t.Logf("failed to cleanup %s (%v)", resourceType, resourceID)
	}
}

func (c *TestContext) postResult(ctx context.Context) error {
	jsonResult, err := json.Marshal(testCtx.result)
	if err != nil {
		return fmt.Errorf("failed to serialize result: %s", err)
	}

	resp, err := http.Post(
		"http://"+c.e2eServerAddr+"/api/results/"+c.result.ID,
		"application/json",
		bytes.NewBuffer(jsonResult),
	)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("error posting result: %d", resp.StatusCode)
	}
	return nil
}

type roundTripperFunc func(req *http.Request) (*http.Response, error)

func (rt roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return rt(req)
}

func (c *TestContext) roundTrip(next http.RoundTripper) roundTripperFunc {
	return func(req *http.Request) (*http.Response, error) {
		start := time.Now()

		res, err := next.RoundTrip(req)

		go func(start time.Time) {
			if err == context.DeadlineExceeded {
				return
			}

			duration := time.Now().Sub(start)
			var (
				status string
				error  string
			)
			if err != nil {
				status = ""
				error = err.Error()
			} else {
				status = strconv.Itoa(res.StatusCode)
				error = ""
			}
			method := req.Method
			endpoint := lookupEndpoint(req)
			if endpoint == "unknown" {
				fmt.Println(req.URL.String())
			}

			c.requestDuration.WithLabelValues(method, endpoint, status, error).Observe(duration.Seconds())
			// fmt.Printf("request: %s %s (%s) - %fs\n", method, endpoint, status, duration.Seconds())
		}(start)

		return res, err
	}
}
