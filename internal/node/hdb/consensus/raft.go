package consensus

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/eagraf/habitat-new/internal/node/hdb"
	"github.com/eagraf/habitat-new/internal/node/hdb/state"
	"github.com/eagraf/habitat-new/pkg/raft/transport"
	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
	"github.com/rs/zerolog/log"
)

const (
	RetainSnapshotCount = 1000
	RaftTimeout         = 10 * time.Second
)

// ClusterService is an implementation of cluster.ClusterManager
type ClusterService struct {
	host string

	instances map[string]*Cluster

	nodeID string
}

type Cluster struct {
	databaseID   string
	filepath     string
	serverID     string
	address      string
	instance     *raft.Raft
	stateMachine *state.RaftFSMAdapter

	updateChan <-chan hdb.StateUpdate
}

func NewClusterService(host string) *ClusterService {
	cs := &ClusterService{
		host:      host,
		instances: make(map[string]*Cluster),

		nodeID: "node_1",
	}

	return cs
}

func (cs *ClusterService) Start() error {
	return nil
}

// CreateCluster initializes a new Raft cluster, and bootstraps it with this nodes address
func (cs *ClusterService) CreateCluster(databaseID, filepath string, raftFSM *state.RaftFSMAdapter) (*Cluster, error) {
	if _, ok := cs.instances[databaseID]; ok {
		return nil, fmt.Errorf("raft instance for database %s already initialized", databaseID)
	}

	ra, err := setupRaftInstance(databaseID, filepath, raftFSM, true, cs.host)
	if err != nil {
		return nil, fmt.Errorf("failed to setup raft instance: %s", err.Error())
	}

	cluster := &Cluster{
		databaseID:   databaseID,
		filepath:     filepath,
		serverID:     getServerID(databaseID),
		address:      getDatabaseAddress(databaseID),
		instance:     ra,
		stateMachine: raftFSM,
		updateChan:   raftFSM.UpdateChan(),
	}
	cs.instances[databaseID] = cluster

	// block until we receive leadership
	leaderCh := ra.LeaderCh()
	if !<-leaderCh {
		return nil, errors.New("did not receive leadership")
	}

	return cluster, nil
}

func (cs *ClusterService) RemoveCluster(databaseID string) error {
	// TODO
	return nil
}

// JoinCluster requests for this node to join a cluster.
// In this implementation, the address is unused because the leader will begin sending
// heartbeets to this node once its AddNode routine has been called.
func (cs *ClusterService) JoinCluster(databaseID, filepath string, address string, raftFSM *state.RaftFSMAdapter) (<-chan hdb.StateUpdate, error) {
	if _, ok := cs.instances[databaseID]; ok {
		return nil, fmt.Errorf("raft instance for database %s already initialized", databaseID)
	}

	ra, err := setupRaftInstance(databaseID, filepath, raftFSM, false, cs.host)
	if err != nil {
		return nil, fmt.Errorf("failed to setup raft instance: %s", err.Error())
	}

	raftInstance := &Cluster{
		databaseID:   databaseID,
		serverID:     getServerID(databaseID),
		address:      getDatabaseAddress(databaseID),
		instance:     ra,
		stateMachine: raftFSM,
	}
	cs.instances[databaseID] = raftInstance

	return raftFSM.UpdateChan(), nil
}

// RestoreNode restarts this nodes raft instance if it has been stopped. All data is
// restored from snapshots and the write ahead log in stable storage.
func (cs *ClusterService) RestoreNode(databaseID, filepath string, raftFSM *state.RaftFSMAdapter) (*Cluster, error) {
	if _, ok := cs.instances[databaseID]; ok {
		log.Error().Msgf("raft instance for database %s already initialized", databaseID)
	}

	ra, err := setupRaftInstance(databaseID, filepath, raftFSM, false, cs.host)
	if err != nil {
		log.Error().Msgf("failed to setup raft instance: %s", err.Error())
	}

	cluster := &Cluster{
		databaseID:   databaseID,
		serverID:     getServerID(databaseID),
		address:      getDatabaseAddress(databaseID),
		instance:     ra,
		stateMachine: raftFSM,
		updateChan:   raftFSM.UpdateChan(),
	}
	cs.instances[databaseID] = cluster

	return cluster, nil
}

// ProposeTransitions takes a proposed update to community state in the form of a JSON patch,
// and attempts to get other nodes to agree to apply the transition to the state machine.
// If succesfully commited, the updated state should be available via the GetState() call.
func (c *Cluster) ProposeTransitions(transitions []byte) (*hdb.JSONState, error) {
	log.Info().Msgf("applying transition to %s", c.databaseID)

	log.Info().Msg(string(transitions))

	future := c.instance.Apply(transitions, RaftTimeout)

	// future.Error() blocks until the cluster finishes processing this attempted entry
	err := future.Error()
	if err != nil {
		return nil, fmt.Errorf("error applying state transition to community %s: %s", c.databaseID, err)
	}

	newState := future.Response()
	if newState == nil {
		return nil, errors.New("got nil state back from Raft apply future")
	}

	if _, ok := newState.(*hdb.JSONState); !ok {
		return nil, errors.New("state returned by Raft apply future is not JSONState")
	}

	return newState.(*hdb.JSONState), nil
}

// GetState returns the state tracked by the Raft instance's state machine. It returns
// a byte array with a marshaled JSON object inside.
func (c *Cluster) State() ([]byte, error) {
	log.Info().Msgf("getting state for %s", c.databaseID)

	state, err := c.stateMachine.State()
	if err != nil {
		return nil, fmt.Errorf("error getting raft instance's community state: %s", err)
	}

	return state, nil
}

// AddNode adds a new node to a cluster. After a node has been added, it must execute
// the JoinCluster routine to begin listening for state updates.
// nodeID represents a libp2p peer id base58 encoded in this instance
func (c *Cluster) AddNode(nodeID string, address string) error {
	log.Info().Msgf("received request for %s at %s to join %s", nodeID, address, c.databaseID)

	configFuture := c.instance.GetConfiguration()

	for _, srv := range configFuture.Configuration().Servers {
		// If a node already exists with either the joining node's ID or address,
		// that node may need to be removed from the config first.
		if srv.ID == raft.ServerID(nodeID) || srv.Address == raft.ServerAddress(address) {
			// However if *both* the ID and the address are the same, then nothing -- not even
			// a join operation -- is needed.
			if srv.Address == raft.ServerAddress(address) && srv.ID == raft.ServerID(nodeID) {
				return fmt.Errorf("node %s at %s already member of cluster, ignoring join request", nodeID, address)
			}

			future := c.instance.RemoveServer(srv.ID, 0, 0)
			if err := future.Error(); err != nil {
				return fmt.Errorf("error removing existing node %s at %s: %s", nodeID, address, err)
			}
		}
	}

	f := c.instance.AddVoter(raft.ServerID(nodeID), raft.ServerAddress(address), 0, 0)
	if f.Error() != nil {
		return f.Error()
	}

	return nil
}

func (c *Cluster) RemoveNode(databaseID string, nodeID string) error {
	// TODO
	return nil
}

// Internal wrapper of Hashicorp raft stuff
func setupRaftInstance(databaseID, dbPath string, stateMachine *state.RaftFSMAdapter, newCommunity bool, host string) (*raft.Raft, error) {
	log.Info().Msgf("setting up raft instance for node %s at %s", getServerID(databaseID), getDatabaseAddress(databaseID))

	// setup raft folder
	raftDBPath := filepath.Join(dbPath, "raft.db")

	_, err := os.Stat(raftDBPath)
	if errors.Is(err, os.ErrNotExist) {
		err := os.MkdirAll(dbPath, 0700)
		if err != nil {
			return nil, fmt.Errorf("error creating raft directory for new community: %s", err)
		}

		raftDBFile, err := os.OpenFile(raftDBPath, os.O_CREATE|os.O_RDONLY, 0600)
		if err != nil {
			return nil, fmt.Errorf("error creating raft bolt db file: %s", err)
		}
		defer raftDBFile.Close()
	} else if err != nil {
		return nil, err
	}

	// Setup Raft configuration.
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(host)

	// Setup Raft communication.
	httpTransport, err := transport.NewHTTPTransport(getDatabaseAddress(databaseID))
	if err != nil {
		return nil, err
	}

	// Create the snapshot store. This allows the Raft to truncate the log.
	snapshots, err := raft.NewFileSnapshotStore(getDataseRaftDirectory(databaseID), RetainSnapshotCount, os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("file snapshot store: %s", err)
	}

	// Create the log store and stable store.
	var logStore raft.LogStore
	var stableStore raft.StableStore
	boltDB, err := raftboltdb.NewBoltStore(filepath.Join(getDataseRaftDirectory(databaseID), "raft.db"))
	if err != nil {
		return nil, fmt.Errorf("new bolt store: %s", err)
	}
	logStore = boltDB
	stableStore = boltDB

	// Instantiate the Raft systems.
	ra, err := raft.NewRaft(config, stateMachine, logStore, stableStore, snapshots, httpTransport)
	if err != nil {
		return nil, fmt.Errorf("new raft: %s", err)
	}

	// If this node is creating the community, bootstrap the raft cluster as well
	if newCommunity {
		configuration := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      config.LocalID,
					Address: raft.ServerAddress(getDatabaseAddress(databaseID)),
				},
			},
		}
		ra.BootstrapCluster(configuration)
	}

	return ra, nil
}

// Implement the state.Replicator interface

func (c *Cluster) UpdateChannel() <-chan hdb.StateUpdate {
	return c.updateChan
}

func (c *Cluster) Dispatch(transition []byte) (*hdb.JSONState, error) {
	encoded := base64.StdEncoding.EncodeToString(transition)
	return c.ProposeTransitions([]byte(encoded))
}
