package loadbalancer

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/digitalocean/godo"
	"github.com/digitalocean/godo/test/e2e/droplet"
	"github.com/digitalocean/godo/test/e2e/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestLoadBalancer_CRUD(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testCtx, finish := framework.SetupContext(ctx, t)
	defer finish(ctx, t)

	public, _, err := framework.NewSSHKey(testCtx.Rand)
	require.NoError(t, err, "error generating ssh key")
	sshKey, _, err := testCtx.Client.Keys.Create(ctx, &godo.KeyCreateRequest{
		Name:      "e2e-test-key",
		PublicKey: string(ssh.MarshalAuthorizedKey(public)),
	})
	require.NoError(t, err, "error creating ssh key")
	defer testCtx.Cleanup(ctx, t, sshKey)

	createReq := droplet.RandomDropletCreateRequest(ctx, t, testCtx, "e2e-lb-target-droplet")
	t.Logf("testing with target droplet - region: %s size: %s image: %s", createReq.Region, createReq.Size, createReq.Image.Slug)
	createReq.SSHKeys = []godo.DropletCreateSSHKey{{Fingerprint: sshKey.Fingerprint}}

	targetDroplet, _, err := testCtx.Client.Droplets.Create(ctx, createReq)
	require.NoError(t, err, "error creating droplet")
	defer testCtx.Cleanup(ctx, t, targetDroplet)

	dropletCtx, cancel := context.WithTimeout(ctx, droplet.CreateTimeout)
	defer cancel()
	targetDroplet = droplet.WaitForDroplet(dropletCtx, t, testCtx, targetDroplet, []string{"active"})

	var lb *godo.LoadBalancer
	defer func() {
		if lb == nil {
			return
		}
		testCtx.Cleanup(ctx, t, lb)
	}()

	t.Run("create", func(t *testing.T) {
		name := fmt.Sprintf("e2e-lb-%d", +time.Now().UnixNano())
		lb, _, err = testCtx.Client.LoadBalancers.Create(ctx, &godo.LoadBalancerRequest{
			Name:   name,
			Region: targetDroplet.Region.Slug,
			ForwardingRules: []godo.ForwardingRule{
				{
					EntryProtocol:  "http",
					EntryPort:      80,
					TargetProtocol: "http",
					TargetPort:     80,
				},
			},
			DropletIDs: []int{targetDroplet.ID},
		})
		require.NoError(t, err, "error creating load balancer")

		assert.NotEmpty(t, lb.ID)
		assert.Equal(t, name, lb.Name)
		assert.Equal(t, []int{targetDroplet.ID}, lb.DropletIDs)
		assert.Equal(t, []godo.ForwardingRule{
			{
				EntryProtocol:  "http",
				EntryPort:      80,
				TargetProtocol: "http",
				TargetPort:     80,
			},
		}, lb.ForwardingRules)

		createCtx, cancel := context.WithTimeout(ctx, CreateTimeout)
		defer cancel()
		lb = WaitForLoadBalancer(createCtx, t, testCtx, lb, []string{"active"})
		assert.Equal(t, "active", lb.Status)
	})
	t.Run("get", func(t *testing.T) {
		if lb == nil {
			t.Skip("skipping since load balancer never created")
		}

		lb, _, err = testCtx.Client.LoadBalancers.Get(ctx, lb.ID)
		require.NoError(t, err, "error getting load balancer")
		assert.NotNil(t, lb)
	})
	t.Run("list", func(t *testing.T) {
		if lb == nil {
			t.Skip("skipping since load balancer never created")
		}

		lbs, _, err := testCtx.Client.LoadBalancers.List(ctx, &godo.ListOptions{})
		require.NoError(t, err, "error listing load balancers")
		for _, l := range lbs {
			if l.ID == lb.ID {
				return
			}
		}
		assert.Fail(t, "failed to find created load balancer")
	})
	t.Run("update", func(t *testing.T) {
		if lb == nil {
			t.Skip("skipping since load balancer never created")
		}

		tag, _, err := testCtx.Client.Tags.Create(ctx, &godo.TagCreateRequest{Name: "test-tag"})
		require.NoError(t, err, "error creating load balancer target tag")

		newName := lb.Name + "-new-name"
		lb, _, err = testCtx.Client.LoadBalancers.Update(ctx, lb.ID, &godo.LoadBalancerRequest{
			Name:   newName,
			Region: targetDroplet.Region.Slug,
			// XXX For some reason these are need for update but not create...
			Algorithm:   lb.Algorithm,
			HealthCheck: lb.HealthCheck,
			ForwardingRules: []godo.ForwardingRule{
				{
					EntryProtocol:  "http",
					EntryPort:      123,
					TargetProtocol: "http",
					TargetPort:     321,
				},
			},
			DropletIDs: nil,
			Tag:        tag.Name,
		})
		require.NoError(t, err, "error updating load balancer")

		updateCtx, cancel := context.WithTimeout(ctx, UpdateTimeout)
		defer cancel()
		lb = WaitForLoadBalancer(updateCtx, t, testCtx, lb, []string{"active"})

		assert.Equal(t, newName, lb.Name)
		assert.Empty(t, lb.DropletIDs)
		assert.Equal(t, tag.Name, lb.Tag)
		assert.Equal(t, []godo.ForwardingRule{
			{
				EntryProtocol:  "http",
				EntryPort:      123,
				TargetProtocol: "http",
				TargetPort:     321,
			},
		}, lb.ForwardingRules)
	})
	t.Run("delete", func(t *testing.T) {
		if lb == nil {
			t.Skip("skipping since load balancer never created")
		}

		_, err := testCtx.Client.LoadBalancers.Delete(ctx, lb.ID)
		require.NoError(t, err, "error deleting load balancer")
	})
}
