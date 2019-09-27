package droplet

import (
	"context"
	"testing"
	"time"

	"github.com/digitalocean/godo"
	"github.com/digitalocean/godo/test/e2e/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestDroplet_SSH(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testCtx, finish := framework.SetupContext(ctx, t)
	defer finish(ctx, t)

	createReq := RandomDropletCreateRequest(ctx, t, testCtx, "e2e-droplet-ssh-rmetadata")
	t.Logf("testing with - region: %s size: %s image: %s", createReq.Region, createReq.Size, createReq.Image.Slug)

	public, sshClientConfig, err := framework.NewSSHKey(testCtx.Rand)
	require.NoError(t, err, "error generating ssh key")
	sshKey, _, err := testCtx.Client.Keys.Create(ctx, &godo.KeyCreateRequest{
		Name:      "e2e-droplet-ssh",
		PublicKey: string(ssh.MarshalAuthorizedKey(public)),
	})
	require.NoError(t, err, "error creating ssh key")
	defer testCtx.Cleanup(ctx, t, sshKey)
	createReq.SSHKeys = []godo.DropletCreateSSHKey{{Fingerprint: sshKey.Fingerprint}}

	droplet, _, err := testCtx.Client.Droplets.Create(ctx, createReq)
	require.NoError(t, err, "error creating droplet")
	defer testCtx.Cleanup(ctx, t, droplet)

	ctx, cancel := context.WithTimeout(ctx, CreateTimeout)
	defer cancel()
	droplet = WaitForDroplet(ctx, t, testCtx, droplet, []string{"active"})

	t.Run("rmetadata ", func(t *testing.T) {
		// NOTE(nan) the droplet takes a bit of time before sshd actually is up and running.
		time.Sleep(1 * time.Minute)
		dropletPublicIP, _ := droplet.PublicIPv4()
		t.Logf("attempting to connect to %s:22", dropletPublicIP)
		client, err := ssh.Dial("tcp", dropletPublicIP+":22", sshClientConfig)
		require.NoError(t, err, "error dialing droplet ssh")

		session, err := client.NewSession()
		require.NoError(t, err, "error creating droplet ssh session")
		defer session.Close()
		out, err := session.CombinedOutput("curl -s http://169.254.169.254/metadata/v1/hostname")
		t.Logf("hostname lookup:\n%s", out)
		require.NoError(t, err, "error looking up hostname via rmetadata")
		assert.Equal(t, "e2e-droplet-ssh-rmetadata", string(out))

		session, err = client.NewSession()
		require.NoError(t, err, "error creating droplet ssh session")
		defer session.Close()
		out, err = session.CombinedOutput("curl -s http://169.254.169.254/metadata/v1/interfaces/public/0/ipv4/address")
		t.Logf("ipv4 lookup:\n%s", out)
		require.NoError(t, err, "error looking up ipv4 via rmetadata")
		assert.Equal(t, dropletPublicIP, string(out))
	})
}
