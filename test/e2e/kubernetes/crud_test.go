package kubernetes

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/digitalocean/godo"
	"github.com/digitalocean/godo/test/e2e/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKubernetes_CRUD(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testCtx, finish := framework.SetupContext(ctx, t)
	defer finish(ctx, t)

	var (
		cluster *godo.KubernetesCluster
		err     error
	)
	defer func() {
		if cluster == nil {
			return
		}
		testCtx.Cleanup(ctx, t, cluster)
	}()

	t.Run("create", func(t *testing.T) {
		options, _, err := testCtx.Client.Kubernetes.GetOptions(ctx)
		require.NoError(t, err, "error getting options for cluster")

		region := options.Regions[testCtx.Rand.Int()%len(options.Regions)]
		version := options.Versions[testCtx.Rand.Int()%len(options.Versions)]
		size := options.Sizes[testCtx.Rand.Int()%len(options.Sizes)]

		name := fmt.Sprintf("e2e-doks-%d", time.Now().UnixNano())
		cluster, _, err = testCtx.Client.Kubernetes.Create(ctx, &godo.KubernetesClusterCreateRequest{
			Name:        name,
			RegionSlug:  region.Slug,
			VersionSlug: version.Slug,
			NodePools: []*godo.KubernetesNodePoolCreateRequest{
				{
					Name:  "e2e-doks-pool-1",
					Size:  size.Slug,
					Count: 3,
				},
			},
		})
		require.NoError(t, err, "error creating cluster")

		createCtx, cancel := context.WithTimeout(ctx, CreateTimeout)
		defer cancel()
		cluster = WaitForKubernetesCluster(createCtx, t, testCtx, cluster, []godo.KubernetesClusterStatusState{godo.KubernetesClusterStatusRunning})

		assert.NotEmpty(t, cluster.ID)
		assert.Equal(t, name, cluster.Name)
	})
	t.Run("get", func(t *testing.T) {
		if cluster == nil {
			t.Skip("skipping since kubernetes cluster never created")
		}

		cluster, _, err = testCtx.Client.Kubernetes.Get(ctx, cluster.ID)
		require.NoError(t, err, "error getting kubernetes cluster")
		assert.NotNil(t, cluster)
	})
	t.Run("list", func(t *testing.T) {
		if cluster == nil {
			t.Skip("skipping since kubernetes cluster never created")
		}

		clusters, _, err := testCtx.Client.Kubernetes.List(ctx, &godo.ListOptions{})
		require.NoError(t, err, "error listing kubernetes clusters")
		for _, c := range clusters {
			if c.ID == cluster.ID {
				return
			}
		}
		assert.Fail(t, "failed to find created kubernetes cluster")
	})
	t.Run("update", func(t *testing.T) {
		if cluster == nil {
			t.Skip("skipping since kubernetes cluster never created")
		}

		newName := cluster.Name + "-new-name"
		cluster, _, err = testCtx.Client.Kubernetes.Update(ctx, cluster.ID, &godo.KubernetesClusterUpdateRequest{
			Name: newName,
		})
		require.NoError(t, err, "error updating kubernetes cluster")
		assert.Equal(t, newName, cluster.Name)
	})
	t.Run("delete", func(t *testing.T) {
		if cluster == nil {
			t.Skip("skipping since kubernetes cluster never created")
		}

		_, err := testCtx.Client.Kubernetes.Delete(ctx, cluster.ID)
		require.NoError(t, err, "error deleting kubernetes cluster")
	})
}
