package controller

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	types "github.com/eagraf/habitat-new/core/api"

	"github.com/eagraf/habitat-new/internal/node/config"
	"github.com/rs/zerolog/log"
)

type PDSClientI interface {
	GetInviteCode(nodeConfig *config.NodeConfig) (string, error)
	CreateAccount(nodeConfig *config.NodeConfig, email, handle, password, inviteCode string) (types.PDSCreateAccountResponse, error)
}

type PDSClient struct {
}

func (p *PDSClient) GetInviteCode(nodeConfig *config.NodeConfig) (string, error) {
	// Make http request to PDS get invite code endpoint
	pdsURL := fmt.Sprintf("http://%s:%s/xrpc/com.atproto.server.createInviteCode", "host.docker.internal", "5001")

	req, err := http.NewRequest(http.MethodPost, pdsURL, bytes.NewBuffer([]byte("{\"useCount\": 1}")))
	if err != nil {
		return "", err
	}

	// Add PDS admin authentication info to headers using HTTP Basic Auth.
	log.Info().Msgf("header: %s", basicAuthHeader(nodeConfig.PDSAdminUsername(), nodeConfig.PDSAdminPassword()))
	req.Header.Add("Authorization", basicAuthHeader(nodeConfig.PDSAdminUsername(), nodeConfig.PDSAdminPassword()))
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	// Parse the response body to get the invite code
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("PDS returned status code for createInviteCode %d: %s", resp.StatusCode, string(body))
	}

	var inviteResponse types.PDSInviteCodeResponse
	err = json.Unmarshal(body, &inviteResponse)
	if err != nil {
		return "", err
	}

	return inviteResponse.Code, nil
}

func (p *PDSClient) CreateAccount(nodeConfig *config.NodeConfig, email, handle, password, inviteCode string) (types.PDSCreateAccountResponse, error) {
	// Make http request to PDS create account endpoint
	pdsURL := fmt.Sprintf("http://%s:%s/xrpc/com.atproto.server.createAccount", "host.docker.internal", "5001")

	reqBody := types.PDSCreateAccountRequest{
		Email:      email,
		Handle:     handle,
		Password:   password,
		InviteCode: inviteCode,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, pdsURL, bytes.NewReader([]byte(body)))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("PDS returned status code for createAccount %d: %s", resp.StatusCode, string(respBody))
	}

	var createAccountResponse types.PDSCreateAccountResponse
	err = json.Unmarshal(respBody, &createAccountResponse)
	if err != nil {
		return nil, err
	}

	return createAccountResponse, nil
}

func basicAuthHeader(username, password string) string {
	auth := username + ":" + password
	return fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(auth)))
}
