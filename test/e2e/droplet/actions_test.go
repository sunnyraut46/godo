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

func TestDroplet_Actions(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testCtx, finish := framework.SetupContext(ctx, t)
	defer finish(ctx, t)

	createReq := RandomDropletCreateRequest(ctx, t, testCtx, "e2e-droplet-actions-power-off")
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

	droplet, _, err := testCtx.Client.Droplets.Create(ctx, createReq)
	require.NoError(t, err, "error creating droplet")
	defer testCtx.Cleanup(ctx, t, droplet)

	createCtx, cancel := context.WithTimeout(ctx, CreateTimeout)
	defer cancel()
	droplet = WaitForDroplet(createCtx, t, testCtx, droplet, []string{"active"})

	t.Run("power_off", func(t *testing.T) {
		_, _, err := testCtx.Client.DropletActions.PowerOff(ctx, droplet.ID)
		require.NoError(t, err, "error powering off droplet")

		ctx, cancel := context.WithTimeout(ctx, PowerOffTimeout)
		defer cancel()
		droplet = WaitForDroplet(ctx, t, testCtx, droplet, []string{"off"})

		assert.Equal(t, "off", droplet.Status)
	})

	t.Run("power_on", func(t *testing.T) {
		_, _, err := testCtx.Client.DropletActions.PowerOn(ctx, droplet.ID)
		require.NoError(t, err, "error powering on droplet")

		ctx, cancel := context.WithTimeout(ctx, PowerOnTimeout)
		defer cancel()
		droplet = WaitForDroplet(ctx, t, testCtx, droplet, []string{"active"})

		assert.Equal(t, "active", droplet.Status)
	})
}
