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
./gochat --mode client --bootnode <ip_of_bootnode_server>:9595 --room test
```

Start client B on a different network with supported NAT
```
./gochat --mode client --bootnode <ip_of_bootnode_server>:9595 --room test
```

Both clients should register with the bootnode server and establish direct UDP connection.
Subsequent communication between peers is possible without a bootnode server.

## Docker usage

If you wish to use Docker you can do so by building a container
```
docker-compose build
```
Start the container by running
```
docker run -it --rm gochat_gochat --mode client --bootnode <ip_of_bootnode_server>:9595 --room test
```
Note: Docker compose up, is not used to start the container, due to limitations of docker-compose and way it redirects output from terminal.

By default Docker will use environment values specified in your .env file.

## Configuration options
Configuration values are defined by the following hierachy

1. Command line arguments (flags)
2. Environment variables (.env file)
3. Default values

For the list of configuration options with example values please check example env file (**.env.example**)
or consult following table.


| Env variable       | Description                                | Flag       | Default Value |
|:------------------:|:------------------------------------------:|:----------:|:-------------:|
| LOCAL_UDP_PORT     | UDP port for outgoing/incoming messages    | --port     | 4545          |
| BOOTNODE_UDP_PORT  | UDP port of bootnode                       |            | 9595          |
| MODE               | Mode (client or bootnode)                  | --mode     | client        |
| BOOTNODE           | IP/port of the bootnode                    | --bootnode |               |
| ROOM               | Room to join                               | --room     |               |
| LISTEN             | Listening ip/port of bootnode              | --listen   | 0.0.0.0:9595  |
| HEARTBEAT_INTERVAL | Frequency of sending heartbeat to bootnode |            | 10 (seconds)  |
| PEER_TIMEOUT       | Time before disconnect                     |            | 15 (seconds)  |


## Supported NATs

- Full Cone NAT
- Restricted Cone NAT
- Port Restricted Cone NAT

Symmetric NATs are **not supported** due to dynamic IP address/port pairs for outgoing connections.
