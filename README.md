# GoChat - p2p group chat

GoChat is terminal-based p2p chat application using UDP hole punching.

![Demo](/demo.gif "Demo")

## Features

### P2P communication over UDP protocol

Direct peer to peer communication over UDP protocol. UDP connection is established by the use of UDP hole punching technique.

### E2E encryption

Messages between peers are encrypted end to end. Using AES encryption and Diffie–Hellman key exchange.

### Communication channels (rooms)

Clients can communicate over any different channels aka rooms. Clients can join rooms by specifying room identifier, defined by an arbitrary string.

### Peer discovery and connection management

Rendezvous server keeps a list of all active peers on the network, grouped by channels.
Liveness of the node is kept using the Heartbeat mechanism. Clients periodically send a heartbeat to the rendezvous server updating the timestamp of their last heartbeat.

Server checks for state of last heartbeat and removes nodes that were not active in the last 15 seconds.

Once a new client connects it is added to the table of active peers and its presence is announced to all other existing active peers on the network. Same applies for disconnects.


### Gracefull disconnects

Clients can handle gracefull disconnects notifying bootnode of peer disconnection from the p2p network. Bootnode removes peer from the list of active peers and relays that information to all active peers on the network.

## Usage

Compile binary using makefile

```
make
```

Start a bootnode (aka. rendezvous server) on a publicly accessible server.

```
./gochat --mode bootnode --listen 0.0.0.0:9595
```

Start client A from a machine behind supported NAT
```
go run main.go --mode client --bootnode <ip_of_bootnode_server>:9595 --room test1
```

Start client B on a different network with supported NAT
```
go run main.go --mode client --bootnode <ip_of_bootnode_server>:9595 --room test1
```

Both clients should register with the bootnode server and establish direct UDP connection.
Subsequent communication between peers is possible without a bootnode server.

## Supported NATs

- Full Cone NAT
- Restricted Cone NAT
- Port Restricted Cone NAT

Symmetric NATs are **not supported** due to dynamic IP address/port pairs for outgoing connections.
