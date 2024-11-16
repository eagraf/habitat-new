package fishtail

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	types "github.com/eagraf/habitat-new/core/api"
	"github.com/eagraf/habitat-new/internal/node/config"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

// Create a mock http client that records requests instead of sending them
type mockTransport struct {
	requests    []*http.Request
	statusCode  int
	clientError error
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	m.requests = append(m.requests, req)

	if m.clientError != nil {
		return nil, m.clientError
	}

	return &http.Response{
		StatusCode: m.statusCode,
		Body:       io.NopCloser(bytes.NewBufferString("")),
	}, nil
}

func TestATProtoEventPublisher(t *testing.T) {
	v := viper.New()
	nodeConfig, err := config.NewTestNodeConfig(v)
	require.NoError(t, err)

	transport := &mockTransport{
		requests:   make([]*http.Request, 0),
		statusCode: http.StatusOK,
	}

	// Create test client with mock transport
	testClient := &http.Client{
		Transport: transport,
	}

	// Verify request was recorded but not sent
	require.Equal(t, 0, len(transport.requests))

	testPublisher := &ATProtoEventPublisher{
		nodeConfig:    nodeConfig,
		subscriptions: make(map[string][]string),
		httpClient:    testClient,
	}

	testPublisher.AddSubscription("app.bsky.feed.like", "http://host.docker.internal:6000/api/v1/ingest")
	testPublisher.AddSubscription("com.habitat.pouch.link", "http://host.docker.internal:6000/api/v1/ingest")
	require.Equal(t, 1, len(testPublisher.GetSubscriptions("app.bsky.feed.like")))
	require.Equal(t, 1, len(testPublisher.GetSubscriptions("com.habitat.pouch.link")))

	testChain := &IngestedRecordChain{
		Collection: "app.bsky.feed.like",
		Records: []*types.PDSGetRecordResponse{
			{
				URI:   "at://did:plc:l3weetptg3ulkbbn7w5zpu2r/app.bsky.feed.like/123",
				CID:   "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3d4q6ll37yxxul4m",
				Value: map[string]interface{}{"test": "data"},
			},
		},
	}

	// Test publishing a record
	errs, err := testPublisher.Publish(testChain)
	require.NoError(t, err)
	require.NotNil(t, errs)
	require.Equal(t, 0, len(errs))
	require.Equal(t, 1, len(transport.requests))
	req := transport.requests[0]
	require.Equal(t, "http://host.docker.internal:6000/api/v1/ingest", req.URL.String())

	// Get request body
	body, err := io.ReadAll(req.Body)
	require.NoError(t, err)

	// Parse the body as JSON
	var bodyChain IngestedRecordChain
	err = json.Unmarshal(body, &bodyChain)
	require.NoError(t, err)

	require.Equal(t, "app.bsky.feed.like", bodyChain.Collection)
	require.Equal(t, "at://did:plc:l3weetptg3ulkbbn7w5zpu2r/app.bsky.feed.like/123", bodyChain.Records[0].URI)
	require.Equal(t, "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3d4q6ll37yxxul4m", bodyChain.Records[0].CID)
	require.Equal(t, map[string]interface{}{"test": "data"}, bodyChain.Records[0].Value)

	// Test an ingestion failure. Currently, we just log it.
	failingTransport := &mockTransport{
		requests:   make([]*http.Request, 0),
		statusCode: http.StatusInternalServerError,
	}

	testPublisher.httpClient.Transport = failingTransport
	errs, err = testPublisher.Publish(testChain)
	require.NoError(t, err)
	require.NotNil(t, errs)
	require.Equal(t, 1, len(errs))
	require.Equal(t, 1, len(failingTransport.requests))

	// Test a client error
	failingTransport.clientError = fmt.Errorf("test client error")
	errs, err = testPublisher.Publish(testChain)
	require.NoError(t, err)
	require.NotNil(t, errs)
	require.Equal(t, 1, len(errs))

	// Test no subscriptions for this collection
	testChain.Collection = "app.bsky.feed.post"
	errs, err = testPublisher.Publish(testChain)
	require.NoError(t, err)
	require.Nil(t, errs)
	require.Equal(t, 0, len(errs))
}
