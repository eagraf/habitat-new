package transport

import (
	"encoding/json"
	"fmt"

	"github.com/hashicorp/raft"
)

// Constants for the various Raft message types. Summary:
// Append Entries: Append to the shared log, updating the replicated state machine
// Request Vote: Request vote messages are sent when an election is ongoing. If the message is acknowleged, it counts as a vote for the sending node
// A very good visualization: http://thesecretlivesofdata.com/raft/
// The other two message types are utility messages that are specific to the Hashicorp raft implementation
const (
	rpcAppendEntries uint8 = iota
	rpcRequestVote
	rpcInstallSnapshot
	rpcTimeoutNow
)

// RaftRequest and RatResponse allow for us to serialize each message type and allow
// us to figure out the right type to unmarshal into once the message is received.
type RaftRequest struct {
	RPCType uint8  `json:"rpc_type"`
	Args    []byte `json:"args"`
}

type RaftResponse struct {
	RPCType uint8  `json:"rpc_type"`
	Resp    []byte `json:"resp"`
}

func unmarshalRPCRequest(rpcType uint8, buf []byte) (interface{}, error) {
	switch rpcType {
	case rpcAppendEntries:
		var res raft.AppendEntriesRequest
		err := json.Unmarshal(buf, &res)
		if err != nil {
			return nil, err
		}
		return &res, nil
	case rpcRequestVote:
		var res raft.RequestVoteRequest
		err := json.Unmarshal(buf, &res)
		if err != nil {
			return nil, err
		}
		return &res, nil
	case rpcInstallSnapshot:
		var res raft.InstallSnapshotRequest
		err := json.Unmarshal(buf, &res)
		if err != nil {
			return nil, err
		}
		return &res, nil
	case rpcTimeoutNow:
		var res raft.TimeoutNowRequest
		err := json.Unmarshal(buf, &res)
		if err != nil {
			return nil, err
		}
		return &res, nil
	default:
		return nil, fmt.Errorf("%d is not a valid rpc type", rpcType)
	}
}

func unmarshalRPCResponse(rpcType uint8, buf []byte, resp interface{}) error {
	switch rpcType {
	case rpcAppendEntries:
		err := json.Unmarshal(buf, resp)
		if err != nil {
			return err
		}
	case rpcRequestVote:
		err := json.Unmarshal(buf, resp)
		if err != nil {
			return err
		}
	case rpcInstallSnapshot:
		err := json.Unmarshal(buf, resp)
		if err != nil {
			return err
		}
	case rpcTimeoutNow:
		err := json.Unmarshal(buf, resp)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("%d is not a valid rpc type", rpcType)
	}

	return nil
}
