package kubernetes

import (
	"context"
	"testing"
	"time"

	"github.com/digitalocean/godo"
	"github.com/digitalocean/godo/test/e2e/framework"
	"github.com/stretchr/testify/require"
)

var (
	CreateTimeout = 10 * time.Minute
)

func WaitForKubernetesCluster(ctx context.Context, t *testing.T, testCtx *framework.TestContext, cluster *godo.KubernetesCluster, states []godo.KubernetesClusterStatusState) *godo.KubernetesCluster {
	t.Helper()

	t.Logf("waiting for kubernetes cluster %s to become ready", cluster.ID)

	var err error
	for {
		cluster, _, err = testCtx.Client.Kubernetes.Get(ctx, cluster.ID)
		if err == context.DeadlineExceeded {
			require.FailNow(t, "kubernetes cluster never became ready")
		}
		require.NoError(t, err, "failed to get kubernetes cluster")

		// t.Logf("kubernetes cluster status: %s", cluster.Status.State)
		for _, state := range states {
			if cluster.Status.State == state {
				return cluster
			}
		}

		select {
		case <-time.After(5 * time.Second):
		case <-ctx.Done():
			require.FailNow(t, "kubernetes cluster never became ready")
		}
	}
}
