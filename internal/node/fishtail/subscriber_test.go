package fishtail

import (
	"testing"

	"github.com/eagraf/habitat-new/internal/node/config"
	ctrl_mocks "github.com/eagraf/habitat-new/internal/node/controller/mocks"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/core/state/node/test_helpers"
)

func TestFishtailIngestExecutor_Execute(t *testing.T) {
	ctrl := gomock.NewController(t)

	nc := ctrl_mocks.NewMockNodeController(ctrl)

	v := viper.New()
	nodeConfig, err := config.NewTestNodeConfig(v)
	require.NoError(t, err)
	publisher := NewATProtoEventPublisher(nodeConfig)
	executor := NewFishtailIngestExecutor(nil, nc, publisher)

	startProcessStateUpdate, err := test_helpers.StateUpdateTestHelper(&node.ProcessStartTransition{
		AppID: "app1",
	}, &node.State{
		AppInstallations: map[string]*node.AppInstallationState{
			"app1": {
				AppInstallation: &node.AppInstallation{
					UserID: "0",
					ID:     "app1",
					Package: node.Package{
						Driver: "test",
					},
				},
			},
		},
		ReverseProxyRules: &map[string]*node.ReverseProxyRule{
			"app1": {
				AppID:  "app1",
				Type:   node.ProxyRuleFishtailIngest,
				Target: "http://localhost:8080",
				FishtailIngestConfig: &node.FishtailIngestConfig{
					SubscribedCollections: []*node.FishtailSubscription{
						{
							Lexicon: "app.bsky.feed.post",
						},
					},
				},
			},
		},
	})
	assert.NoError(t, err)

	shouldExecute, err := executor.ShouldExecute(startProcessStateUpdate)
	assert.NoError(t, err)
	assert.Equal(t, true, shouldExecute)

	// Execute the transition
	err = executor.Execute(startProcessStateUpdate)
	assert.NoError(t, err)

	// Verify subscription was added
	subscriptions := publisher.GetSubscriptions("app.bsky.feed.post")
	assert.Equal(t, 1, len(subscriptions))
	assert.Equal(t, "http://localhost:8080", subscriptions[0])
}

func TestFishtailIngestRestorer(t *testing.T) {
	nodeConfig := &config.NodeConfig{}
	publisher := NewATProtoEventPublisher(nodeConfig)
	restorer := NewFishtailIngestRestorer(nodeConfig, publisher)

	// Create a state update that represents a restored node with a running process
	restoreStateUpdate, err := test_helpers.StateUpdateTestHelper(&node.InitalizationTransition{}, &node.State{
		Processes: map[string]*node.ProcessState{
			"proc1": {
				Process: &node.Process{
					ID:    "proc1",
					AppID: "app1",
				},
				State: node.ProcessStateRunning,
			},
		},
		AppInstallations: map[string]*node.AppInstallationState{
			"app1": {
				AppInstallation: &node.AppInstallation{
					ID: "app1",
				},
			},
		},
		ReverseProxyRules: &map[string]*node.ReverseProxyRule{
			"rule1": {
				AppID:  "app1",
				Type:   node.ProxyRuleFishtailIngest,
				Target: "http://localhost:8080",
				FishtailIngestConfig: &node.FishtailIngestConfig{
					SubscribedCollections: []*node.FishtailSubscription{
						{
							Lexicon: "app.bsky.feed.post",
						},
						{
							Lexicon: "app.bsky.feed.like",
						},
					},
				},
			},
		},
	})
	assert.NoError(t, err)

	// Restore the state
	err = restorer.Restore(restoreStateUpdate)
	assert.NoError(t, err)

	// Verify subscriptions were restored
	postSubscriptions := publisher.GetSubscriptions("app.bsky.feed.post")
	assert.Equal(t, 1, len(postSubscriptions))
	assert.Equal(t, "http://localhost:8080", postSubscriptions[0])

	likeSubscriptions := publisher.GetSubscriptions("app.bsky.feed.like")
	assert.Equal(t, 1, len(likeSubscriptions))
	assert.Equal(t, "http://localhost:8080", likeSubscriptions[0])
}
