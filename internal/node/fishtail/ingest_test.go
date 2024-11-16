package fishtail

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/bluesky-social/indigo/atproto/syntax"
	types "github.com/eagraf/habitat-new/core/api"
	"github.com/eagraf/habitat-new/internal/node/config"
	"github.com/eagraf/habitat-new/internal/node/controller"
	"github.com/fxamacker/cbor/v2"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartIngestion(t *testing.T) {
	// Create test CBOR data representing a like record
	likeRecord := map[string]interface{}{
		"$type": "app.bsky.feed.like",
		"subject": map[string]interface{}{
			"cid": "bafyreicacaobexeihly7u36lyeoxu4fpaawmxayvbo4xbd2quehdvpo6ru",
			"uri": "at://did:plc:ldb6hx3aef2vhrctg2xdepjw/app.bsky.feed.post/3lax6iocldo2q",
		},
		"createdAt": "2024-11-15T02:20:45.352Z",
	}

	var buf bytes.Buffer
	encoder := cbor.NewEncoder(&buf)
	err := encoder.Encode(likeRecord)
	require.NoError(t, err)

	initialCBOR := buf.Bytes()
	initialCID := "bafyreifpjbvvf2t7xsr2enwnuldbbs7wcet2c5njs7pol6xu4ugvzgr3wi"
	initialURI := "at://did:plc:l3weetptg3ulkbbn7w5zpu2r/app.bsky.feed.like/3laxcvii5rs2e"

	v := viper.New()
	nodeConfig, err := config.NewTestNodeConfig(v)
	require.NoError(t, err)

	transport := &mockTransport{
		requests:   make([]*http.Request, 0),
		statusCode: http.StatusOK,
		responseBody: `{
			"uri": "at://did:plc:ldb6hx3aef2vhrctg2xdepjw/app.bsky.feed.post/3lax6iocldo2q",
			"cid": "bafyreicacaobexeihly7u36lyeoxu4fpaawmxayvbo4xbd2quehdvpo6ru",
			"value": {
				"$type": "app.bsky.feed.post",
				"text": "Test post",
				"createdAt": "2024-11-15T02:20:45.352Z"
			}
		}`,
	}

	client := &http.Client{
		Transport: transport,
	}

	publisher := &ATProtoEventPublisher{
		nodeConfig:    nodeConfig,
		subscriptions: make(map[string][]string),
		httpClient:    client,
	}

	recordChainIngester := NewRecordChainIngestor(
		initialCBOR,
		initialCID,
		initialURI,
		publisher,
	)
	recordChainIngester.pdsClient = controller.NewPDClientWithHTTPClient(nodeConfig, client)

	err = recordChainIngester.StartIngestion()
	require.NoError(t, err)

	// Verify the initial record was ingested
	require.Equal(t, 1, len(recordChainIngester.ingestedRecords))
	require.Equal(t, initialURI, recordChainIngester.ingestedRecords[0].URI)
	require.Equal(t, initialCID, recordChainIngester.ingestedRecords[0].CID)

	// Verify the linked record was enqueued
	require.Equal(t, 1, len(recordChainIngester.toIngest))

	// Assert that no PDS calls were made
	require.Equal(t, 0, len(transport.requests))
}

func TestIngestNextAndEnqueueRecord(t *testing.T) {
	atURI := syntax.ATURI("at://did:plc:l3weetptg3ulkbbn7w5zpu2r/app.bsky.feed.like/3laxcvii5rs2e")

	v := viper.New()
	nodeConfig, err := config.NewTestNodeConfig(v)
	require.NoError(t, err)

	transport := &mockTransport{
		requests:   make([]*http.Request, 0),
		statusCode: http.StatusOK,
		responseBody: `{
            "uri": "at://did:plc:l3weetptg3ulkbbn7w5zpu2r/app.bsky.feed.like/3laxcvii5rs2e",
            "cid": "bafyreifpjbvvf2t7xsr2enwnuldbbs7wcet2c5njs7pol6xu4ugvzgr3wi",
            "value": {
                "$type": "app.bsky.feed.like",
                "subject": {
                    "cid": "bafyreicacaobexeihly7u36lyeoxu4fpaawmxayvbo4xbd2quehdvpo6ru",
                    "uri": "at://did:plc:ldb6hx3aef2vhrctg2xdepjw/app.bsky.feed.post/3lax6iocldo2q"
                },
                "createdAt": "2024-11-15T02:20:45.352Z"
            }
        }`,
	}

	client := &http.Client{
		Transport: transport,
	}

	recordChainIngester := &RecordChainIngester{
		seen:      make(map[string]interface{}),
		toIngest:  make(chan syntax.ATURI, 1000),
		pdsClient: controller.NewPDClientWithHTTPClient(nodeConfig, client),
	}

	require.Equal(t, 0, len(recordChainIngester.toIngest))

	ok := recordChainIngester.EnqueueRecord(atURI)
	require.Equal(t, true, ok)
	require.Equal(t, 1, len(recordChainIngester.toIngest))

	ok, err = recordChainIngester.IngestNext()
	require.NoError(t, err)
	require.Equal(t, true, ok)
	require.Equal(t, 1, len(transport.requests))

	assert.Equal(t, 1, len(recordChainIngester.toIngest))

	// Check that we have one ingested record now.
	require.Equal(t, 1, len(recordChainIngester.ingestedRecords))
	require.Equal(t, atURI.String(), recordChainIngester.ingestedRecords[0].URI)

	// Manually pop the next UR from the channel so we can inspect it.
	nextURI := <-recordChainIngester.toIngest
	assert.Equal(t, "at://did:plc:ldb6hx3aef2vhrctg2xdepjw/app.bsky.feed.post/3lax6iocldo2q", nextURI.String())

	assert.Equal(t, 2, len(recordChainIngester.seen))
	assert.Equal(t, true, recordChainIngester.seen[nextURI.String()])

	// Test seen
	added := recordChainIngester.EnqueueRecord(nextURI)
	assert.Equal(t, false, added)

	// Test enqueueing something unseen
	unseenURI := syntax.ATURI("at://did:plc:l3weetptg3ulkbbn7w5zpu2r/app.bsky.feed.post/3lax6iocldo2q")
	added = recordChainIngester.EnqueueRecord(unseenURI)
	assert.Equal(t, true, added)
	assert.Equal(t, 3, len(recordChainIngester.seen))
	assert.Equal(t, true, recordChainIngester.seen[unseenURI.String()])

	assert.Equal(t, 1, len(recordChainIngester.toIngest))
	assert.Equal(t, unseenURI, <-recordChainIngester.toIngest)
	assert.Equal(t, 0, len(recordChainIngester.toIngest))

	// Test IngestNext when there are no more records to ingest
	ok, err = recordChainIngester.IngestNext()
	require.NoError(t, err)
	assert.Equal(t, false, ok)
	assert.Equal(t, 0, len(recordChainIngester.toIngest))
}

func TestFinishIngestion(t *testing.T) {
	v := viper.New()
	nodeConfig, err := config.NewTestNodeConfig(v)
	require.NoError(t, err)

	transport := &mockTransport{
		requests:   make([]*http.Request, 0),
		statusCode: http.StatusOK,
	}

	testClient := &http.Client{
		Transport: transport,
	}

	testPublisher := &ATProtoEventPublisher{
		nodeConfig:    nodeConfig,
		subscriptions: make(map[string][]string),
		httpClient:    testClient,
	}

	testPublisher.AddSubscription("app.bsky.feed.like", "http://host.docker.internal:6000/api/v1/ingest")

	recordChainIngester := &RecordChainIngester{
		initialURI: "at://did:plc:l3weetptg3ulkbbn7w5zpu2r/app.bsky.feed.like/123",
		ingestedRecords: []*types.PDSGetRecordResponse{
			{
				URI: "at://did:plc:l3weetptg3ulkbbn7w5zpu2r/app.bsky.feed.like/123",
				CID: "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3d4q6ll37yxxul4m",
				Value: map[string]interface{}{
					"$type":     "app.bsky.feed.like",
					"subject":   map[string]interface{}{"uri": "at://test.com"},
					"createdAt": "2024-01-01T00:00:00Z",
				},
			},
			{
				URI: "at://did:plc:l3weetptg3ulkbbn7w5zpu2r/app.bsky.feed.like/456",
				CID: "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3d4q6ll37yxxul4n",
				Value: map[string]interface{}{
					"$type":     "app.bsky.feed.like",
					"subject":   map[string]interface{}{"uri": "at://test2.com"},
					"createdAt": "2024-01-01T00:00:01Z",
				},
			},
		},
		atProtoEventPublisher: testPublisher,
	}

	err = recordChainIngester.FinishIngestion()
	require.NoError(t, err)

	// Verify the request was made
	require.Equal(t, 1, len(transport.requests))
	req := transport.requests[0]
	require.Equal(t, "http://host.docker.internal:6000/api/v1/ingest", req.URL.String())

	// Verify request body
	body, err := io.ReadAll(req.Body)
	require.NoError(t, err)

	var publishedChain IngestedRecordChain
	err = json.Unmarshal(body, &publishedChain)
	require.NoError(t, err)

	// Verify chain contents
	require.Equal(t, "app.bsky.feed.like", publishedChain.Collection)
	require.Equal(t, recordChainIngester.initialURI, publishedChain.InitialRecordURI)
	require.Equal(t, 2, len(publishedChain.Records))
	require.Equal(t, recordChainIngester.ingestedRecords[0].URI, publishedChain.Records[0].URI)
	require.Equal(t, recordChainIngester.ingestedRecords[1].URI, publishedChain.Records[1].URI)
}
