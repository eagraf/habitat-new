package main

import (
	"fmt"
	"net/url"
	"path/filepath"

	"github.com/docker/docker/api/types/mount"
	"github.com/docker/go-connections/nat"
	types "github.com/eagraf/habitat-new/core/api"
	"github.com/eagraf/habitat-new/core/state/node"
	"github.com/eagraf/habitat-new/hdb"
	"github.com/eagraf/habitat-new/internal/node/config"
	"github.com/eagraf/habitat-new/internal/node/constants"
	"github.com/google/uuid"
)

func generatePDSAppConfig(nodeConfig *config.NodeConfig) *types.PostAppRequest {
	pdsMountDir := filepath.Join(nodeConfig.HabitatAppPath(), "pds")

	// TODO @eagraf - unhardcode as much of this as possible
	return &types.PostAppRequest{
		AppInstallation: &node.AppInstallation{
			Name:    "pds",
			Version: "1",
			UserID:  constants.RootUserID,
			Package: node.Package{
				Driver: "docker",
				DriverConfig: map[string]interface{}{
					"env": []string{
						fmt.Sprintf("PDS_HOSTNAME=%s", nodeConfig.Domain()),
						"PDS_DATA_DIRECTORY=/pds",
						"PDS_BLOBSTORE_DISK_LOCATION=/pds/blocks",
						"PDS_PLC_ROTATION_KEY_K256_PRIVATE_KEY_HEX=5290bb1866a03fb23b09a6ffd64d21f6a4ebf624eaa301930eeb81740699239c",
						"PDS_JWT_SECRET=bd6df801372d7058e1ce472305d7fc2e",
						"PDS_ADMIN_PASSWORD=password",
						"PDS_BSKY_APP_VIEW_URL=https://api.bsky.app",
						"PDS_BSKY_APP_VIEW_DID=did:web:api.bsky.app",
						"PDS_REPORT_SERVICE_URL=https://mod.bsky.app",
						"PDS_INVITE_REQUIRED=false",
						"PDS_REPORT_SERVICE_DID=did:plc:ar7c4by46qjdydhdevvrndac",
						"PDS_CRAWLERS=https://bsky.network",
						"DEBUG=t",
					},
					"mounts": []mount.Mount{
						{
							Type:   "bind",
							Source: pdsMountDir,
							Target: "/pds",
						},
					},
					"exposed_ports": []string{"5001"},
					"port_bindings": map[nat.Port][]nat.PortBinding{
						"3000/tcp": {
							{
								HostIP:   "0.0.0.0",
								HostPort: "5001",
							},
						},
					},
				},
				RegistryURLBase:    "registry.hub.docker.com",
				RegistryPackageID:  "ethangraf/pds",
				RegistryPackageTag: "latest",
			},
		},
		ReverseProxyRules: []*node.ReverseProxyRule{
			{
				Type:    "redirect",
				Matcher: "/xrpc",
				Target:  "http://host.docker.internal:5001/xrpc",
			},
		},
	}
}

func generateDefaultReverseProxyRules(frontendDev bool) ([]*node.ReverseProxyRule, error) {
	apiURL, err := url.Parse(fmt.Sprintf("http://localhost:%s", constants.DefaultPortHabitatAPI))
	if err != nil {
		return nil, err
	}

	frontendRule := &node.ReverseProxyRule{
		ID:      "default-rule-frontend",
		Matcher: "", // Root matcher
	}
	if frontendDev {
		// In development mode, we run the frontend in a separate docker container with hot-reloading.
		// As a result, all frontend requests must be forwarde to the frontend container.
		frontendRule.Type = node.ProxyRuleRedirect
		frontendRule.Target = "http://habitat_frontend:8000/"
	} else {
		// In production mode, we embed the frontend into the node binary. That way, we can serve
		// the frontend without needing to set it up on the host machine.
		// TODO @eagraf - evaluate the performance implications of this.
		frontendRule.Type = node.ProxyRuleEmbeddedFrontend
	}

	return []*node.ReverseProxyRule{
		{
			ID:      "default-rule-api",
			Type:    node.ProxyRuleRedirect,
			Matcher: "/habitat/api",
			Target:  apiURL.String(),
		},
		frontendRule,
	}, nil
}

func generateInitState(nodeConfig *config.NodeConfig) (*node.State, error) {
	// TODO this is basically a placeholder until we actually have a way of generating
	// the certificate for the node.
	nodeUUID := uuid.New().String()
	rootCert := nodeConfig.RootUserCertB64()

	initState, err := node.GetEmptyStateForVersion(node.LatestVersion)
	if err != nil {
		return nil, err
	}

	initState.NodeID = nodeUUID
	initState.Users[constants.RootUserID] = &node.User{
		ID:          constants.RootUserID,
		Username:    constants.RootUsername,
		Certificate: rootCert,
	}

	return initState, nil
}

func initTranstitions(initState *node.State, startApps []*types.PostAppRequest, proxyRules []*node.ReverseProxyRule) ([]hdb.Transition, error) {
	// A list of transitions to apply when the node starts up for the first time.
	transitions := []hdb.Transition{
		&node.InitalizationTransition{
			InitState: initState,
		},
	}

	for _, rule := range proxyRules {
		transitions = append(transitions, &node.AddReverseProxyRuleTransition{
			Rule: rule,
		})
	}

	for _, app := range startApps {
		transitions = append(transitions, &node.StartInstallationTransition{
			UserID:                 constants.RootUserID,
			AppInstallation:        app.AppInstallation,
			NewProxyRules:          app.ReverseProxyRules,
			StartAfterInstallation: true,
		})
	}

	return transitions, nil
}
