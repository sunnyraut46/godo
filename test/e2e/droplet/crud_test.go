package droplet

import (
	"context"
	"testing"

	"github.com/digitalocean/godo"
	"github.com/digitalocean/godo/test/e2e/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestDroplet_CRUD(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testCtx, finish := framework.SetupContext(ctx, t)
	defer finish(ctx, t)

	var droplet *godo.Droplet
	defer func() {
		if droplet == nil {
			return
		}
		testCtx.Cleanup(ctx, t, droplet)
	}()

	t.Run("create", func(t *testing.T) {
		createReq := RandomDropletCreateRequest(ctx, t, testCtx, "e2e-droplet-crud-create")
		t.Logf("testing with - region: %s size: %s image: %s", createReq.Region, createReq.Size, createReq.Image.Slug)

		public, _, err := framework.NewSSHKey(testCtx.Rand)
		require.NoError(t, err, "error generating ssh key")
		sshKey, _, err := testCtx.Client.Keys.Create(ctx, &godo.KeyCreateRequest{
			Name:      "e2e-test-key",
			PublicKey: string(ssh.MarshalAuthorizedKey(public)),
		})
		require.NoError(t, err, "error creating ssh key")
		defer testCtx.Cleanup(ctx, t, sshKey)
		createReq.SSHKeys = []godo.DropletCreateSSHKey{{Fingerprint: sshKey.Fingerprint}}

		droplet, _, err = testCtx.Client.Droplets.Create(ctx, createReq)
		require.NoError(t, err, "error creating droplet")

		assert.NotZero(t, droplet.ID)
		assert.Equal(t, createReq.Name, droplet.Name)
		assert.Equal(t, createReq.Region, droplet.Region.Slug)
		assert.Equal(t, createReq.Size, droplet.Size.Slug)
		assert.Equal(t, createReq.Image.Slug, droplet.Image.Slug)

		createCtx, cancel := context.WithTimeout(ctx, CreateTimeout)
		defer cancel()
		droplet = WaitForDroplet(createCtx, t, testCtx, droplet, []string{"active"})
		assert.Equal(t, "active", droplet.Status)
	})

	t.Run("get", func(t *testing.T) {
		if droplet == nil {
			t.Skip("skipping since droplet never created")
		}

		d, _, err := testCtx.Client.Droplets.Get(ctx, droplet.ID)
		require.NoError(t, err, "error getting droplet")

		assert.Equal(t, droplet.ID, d.ID)
		assert.Equal(t, droplet.Name, d.Name)
	})

	t.Run("list", func(t *testing.T) {
		if droplet == nil {
			t.Skip("skipping since droplet never created")
		}

		// TODO(nan) handle paginated case, assuming always less than default page
		// size # of droplets.
		droplets, _, err := testCtx.Client.Droplets.List(ctx, &godo.ListOptions{})
		require.NoError(t, err, "error listing droplets")

		for _, d := range droplets {
			if d.ID == droplet.ID {
				return
			}
		}
		assert.Fail(t, "failed to find created droplet")
	})

	t.Run("delete", func(t *testing.T) {
		if droplet == nil {
			t.Skip("skipping since droplet never created")
		}

		_, err := testCtx.Client.Droplets.Delete(ctx, droplet.ID)
		require.NoError(t, err, "error deleting droplet")
	})
}
