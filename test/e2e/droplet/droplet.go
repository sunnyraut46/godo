package droplet

import (
	"context"
	"testing"
	"time"

	"github.com/digitalocean/godo"
	"github.com/digitalocean/godo/test/e2e/framework"
	"github.com/stretchr/testify/require"
)

var (
	imageSlugs = []string{"ubuntu-18-04-x64"}

	CreateTimeout   = 2 * time.Minute
	PowerOffTimeout = 5 * time.Minute
	PowerOnTimeout  = 5 * time.Minute
)

func RandomDropletCreateRequest(ctx context.Context, t *testing.T, testCtx *framework.TestContext, name string) *godo.DropletCreateRequest {
	regions, _, err := testCtx.Client.Regions.List(context.Background(), &godo.ListOptions{})
	require.NoError(t, err)

	var availableRegions []godo.Region
	for _, region := range regions {
		if !region.Available {
			continue
		}
		availableRegions = append(availableRegions, region)
	}

	region := availableRegions[testCtx.Rand.Int()%len(availableRegions)]
	sizeSlug := region.Sizes[testCtx.Rand.Int()%len(region.Sizes)]
	imageSlug := imageSlugs[testCtx.Rand.Int()%len(imageSlugs)]

	return &godo.DropletCreateRequest{
		Name:   name,
		Region: region.Slug,
		Size:   sizeSlug,
		Image:  godo.DropletCreateImage{Slug: imageSlug},
	}
}

func WaitForDroplet(ctx context.Context, t *testing.T, testCtx *framework.TestContext, droplet *godo.Droplet, dropletStatuses []string) *godo.Droplet {
	t.Helper()

	t.Logf("waiting for droplet %d to become ready", droplet.ID)

	var err error
	for {
		droplet, _, err = testCtx.Client.Droplets.Get(ctx, droplet.ID)
		if err == context.DeadlineExceeded {
			require.FailNow(t, "droplet never became ready")
		}
		require.NoError(t, err, "failed to get droplet")

		// t.Logf("droplet status: %s", droplet.Status)
		dropletReady := false
		for _, status := range dropletStatuses {
			if droplet.Status == status {
				dropletReady = true
				break
			}
		}

		actions, _, err := testCtx.Client.Droplets.Actions(ctx, droplet.ID, &godo.ListOptions{})
		if err == context.DeadlineExceeded {
			require.FailNow(t, "droplet never became ready")
		}
		require.NoError(t, err, "failed to get droplet actions")

		actionsComplete := true
		for _, action := range actions {
			// t.Logf("action %d: %s", action.ID, action.Status)
			if action.Status != godo.ActionCompleted {
				actionsComplete = false
				break
			}
		}

		if dropletReady && actionsComplete {
			return droplet
		}

		select {
		case <-time.After(5 * time.Second):
		case <-ctx.Done():
			require.FailNow(t, "droplet never became ready")
		}
	}
}
