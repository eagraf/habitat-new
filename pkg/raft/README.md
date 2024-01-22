This package implements a basic consensus protocol for Habitat communities, based off of the HashiCorp Raft library. Habitat can run multiple communities on one node, necesitating multiple instances of a Raft agent to run side by side, which influences most of the major design decisions in this package.

## LibP2P Trasport layer
Allows for Raft to be executed over libp2p streams. Conceptually, it works very similar to the `NetTransport` provided by the `raft` library. Incoming raft messages are handled by a libp2p stream handler, and outgoing messages are sent via a basic RPC mechanism.

TODO:
* Implement pipelining entries optimization
* Readd timeouts and stuff to prevent long hangs
* Look into pooling streams/multiplexing to avoid constantley reopening connections
* Forwarding new log entries to the leader
* Test InstallSnapshot stuff/look into why network transport has longer timeouts for it
* Use proto buffers to serialize messages

## HTTP Transport Implementation
`Note: the HTTP tranport layer and the multiplexer is no longer used`

One constraint on the design is that we want each node to expose only one port to the internet for the purpose of sharing essential state with other nodes. To do this, we need to have each community listen for state updates through the Raft protocol on the same port without interfering with each other. To keep communities isolated, we append a path to the server address of the node. Unfortunately, Raft only implements a TCP transport layer which can only handler ip:port formatted server addresses. To get around this, we implement our own transport layer that uses HTTP rather than TCP streams. While this may be slower, it allows us to specify paths that a router can use to determine which community's cluster should be modified.

## Multiplexer
The multiplexer acts as the routing layer that redirects incoming Raft protocol requests to the right instance of the Raft agent. This allows for multiple communities to listen for Raft messages on the same server, which reduces complexity in terms of concurrent servers listening to ports. The multiplexer maintains a map linking community id's to their corresponding http transport instance.

## Finite State Machine
CommunityStateMachine implements the raft.FSM interface, and keeps track of the state of a community. The data is maintained as JSON stored in a byte array, with updates being described as JSON patches (http://jsonpatch.com/).