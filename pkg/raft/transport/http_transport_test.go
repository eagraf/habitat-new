package transport

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
)

func TestRPCMarshaling(t *testing.T) {
	appendEntriesReq := &raft.AppendEntriesRequest{
		RPCHeader: raft.RPCHeader{
			ProtocolVersion: 3,
		},
		PrevLogEntry: 20,
		Entries: []*raft.Log{
			{
				Index: 102,
			},
		},
	}

	aeBuf, err := json.Marshal(appendEntriesReq)
	if err != nil {
		t.Error(err)
	}
	fmt.Println(string(aeBuf))

	res, err := unmarshalRPCRequest(rpcAppendEntries, aeBuf)
	if err != nil {
		t.Error(err)
	}

	fmt.Println(res)
	assert.Equal(t, raft.ProtocolVersion(3), res.(*raft.AppendEntriesRequest).RPCHeader.ProtocolVersion)
	assert.Equal(t, uint64(20), res.(*raft.AppendEntriesRequest).PrevLogEntry)
	assert.Equal(t, uint64(102), res.(*raft.AppendEntriesRequest).Entries[0].Index)
}
