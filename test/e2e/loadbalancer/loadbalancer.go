package loadbalancer

import (
	"context"
	"testing"
	"time"

	"github.com/digitalocean/godo"
	"github.com/digitalocean/godo/test/e2e/framework"
	"github.com/stretchr/testify/require"
)

var (
	CreateTimeout = 5 * time.Minute
	UpdateTimeout = 5 * time.Minute
)

func WaitForLoadBalancer(ctx context.Context, t *testing.T, testCtx *framework.TestContext, lb *godo.LoadBalancer, statuses []string) *godo.LoadBalancer {
	t.Helper()

	t.Logf("waiting for load balancer %s to become ready", lb.ID)

	var err error
	for {
		lb, _, err = testCtx.Client.LoadBalancers.Get(ctx, lb.ID)
		if err == context.DeadlineExceeded {
			require.FailNow(t, "load balancer never became ready")
		}
		require.NoError(t, err, "failed to get load balancer")

		// t.Logf("load balancer status: %s", lb.Status)
		for _, status := range statuses {
			if lb.Status == status {
				return lb
			}
		}

		select {
		case <-time.After(5 * time.Second):
		case <-ctx.Done():
			require.FailNow(t, "load balancer never became ready")
		}
	}
}
