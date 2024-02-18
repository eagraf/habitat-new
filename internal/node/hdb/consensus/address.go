package consensus

import "fmt"

const (
	RaftPort = "7000"
)

// Used by Raft to identify this node in the cluster.
func getServerID(communityID string) string {
	return "habitat_server_1"
}

// Get the address for a specific Raft instance inside a cluster.
func getDatabaseAddress(databaseID string) string {
	//	return fmt.Sprintf("%s://%s/%s", Protocol, getMultiplexerAddress(), communityID)
	return fmt.Sprintf("http://localhost:%s/%s", RaftPort, databaseID)
}
