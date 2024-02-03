package transport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/hashicorp/raft"
)

// The main idea of HTTPTransport is to allow multiple instances of Raft on one machine communicate with peers in their respective clusters
// through a single port on a single process, while remaining isolated from each other.
type HTTPTransport struct {
	sync.RWMutex
	consumeCh chan raft.RPC
	address   raft.ServerAddress

	heartbeatFn     func(raft.RPC)
	heartbeatFnLock sync.Mutex
}

func NewHTTPTransport(address string) (*HTTPTransport, error) {
	return &HTTPTransport{
		RWMutex:         sync.RWMutex{},
		consumeCh:       make(chan raft.RPC),
		address:         raft.ServerAddress(address),
		heartbeatFnLock: sync.Mutex{},
	}, nil
}

func (ht *HTTPTransport) Consumer() <-chan raft.RPC {
	return ht.consumeCh
}

func (ht *HTTPTransport) LocalAddr() raft.ServerAddress {
	return ht.address
}

func (ht *HTTPTransport) AppendEntriesPipeline(id raft.ServerID, target raft.ServerAddress) (raft.AppendPipeline, error) {

	// TODO figure out how pipelining works and implement the optimization
	//return nil, errors.New("HTTPTransport does not support AppendEntriesPipeline, fall back to standard replication")
	return nil, raft.ErrPipelineReplicationNotSupported
}

func (ht *HTTPTransport) AppendEntries(id raft.ServerID, target raft.ServerAddress, args *raft.AppendEntriesRequest, resp *raft.AppendEntriesResponse) error {
	return ht.genericRPC(id, target, rpcAppendEntries, args, resp)
}

func (ht *HTTPTransport) RequestVote(id raft.ServerID, target raft.ServerAddress, args *raft.RequestVoteRequest, resp *raft.RequestVoteResponse) error {
	return ht.genericRPC(id, target, rpcRequestVote, args, resp)
}

func (ht *HTTPTransport) InstallSnapshot(id raft.ServerID, target raft.ServerAddress, args *raft.InstallSnapshotRequest, resp *raft.InstallSnapshotResponse, data io.Reader) error {
	return ht.genericRPC(id, target, rpcInstallSnapshot, args, resp)
}

func (ht *HTTPTransport) EncodePeer(id raft.ServerID, addr raft.ServerAddress) []byte {
	return []byte(addr)
}

func (ht *HTTPTransport) DecodePeer(buf []byte) raft.ServerAddress {
	return raft.ServerAddress(buf)
}

func (ht *HTTPTransport) SetHeartbeatHandler(cb func(rpc raft.RPC)) {
	ht.heartbeatFnLock.Lock()
	defer ht.heartbeatFnLock.Unlock()
	ht.heartbeatFn = cb
}

func (ht *HTTPTransport) TimeoutNow(id raft.ServerID, target raft.ServerAddress, args *raft.TimeoutNowRequest, resp *raft.TimeoutNowResponse) error {
	return ht.genericRPC(id, target, rpcTimeoutNow, args, resp)
}

func (ht *HTTPTransport) genericRPC(id raft.ServerID, target raft.ServerAddress, rpcType uint8, args interface{}, resp interface{}) error {

	buf, err := json.Marshal(args)
	if err != nil {
		return err
	}

	req := &RaftRequest{
		RPCType: rpcType,
		Args:    buf,
	}

	// yolo try to json marshal without using proper serialization format
	encoded, err := json.Marshal(req)
	if err != nil {
		return err
	}

	postResp, err := http.Post(string(target), "application/json", bytes.NewReader(encoded))
	if err != nil {
		return err
	}

	respBody, err := io.ReadAll(postResp.Body)
	if err != nil {
		return err
	}

	if postResp.StatusCode != http.StatusOK {
		return fmt.Errorf("error: got status response %d: %s", postResp.StatusCode, string(respBody))

	}

	var htResp RaftResponse
	err = json.Unmarshal(respBody, &htResp)
	if err != nil {
		return err
	}

	return unmarshalRPCResponse(rpcType, htResp.Resp, resp)
}
