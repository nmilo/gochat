package bootnode

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/nmilo/gochat/message"
)

type PeerConnection struct {
	Address       string
	Conn          net.Addr
	LastHeartbeat time.Time
}

type Room struct {
	Name  string
	Peers map[string]*PeerConnection
}

var rooms = make(map[string]*Room)

const peerTimeout = 15 * time.Second

var mu sync.Mutex

func Start(listen string) {
	addr, _ := net.ResolveUDPAddr("udp", listen)
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		fmt.Println("Error listening:", err)
		return
	}
	defer conn.Close()

	fmt.Printf("Bootnode is listening on %s\n", listen)

	go pruneInactivePeers(conn)

	// Accept connections
	for {
		buffer := make([]byte, 1024)
		n, remoteAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			panic(err)
		}

		// Decode the message from bytes
		msg, err := message.Decode(buffer[:n])
		if err != nil {
			fmt.Println("Error decoding message:", err)
			continue
		}
		fmt.Printf("Received '%s' from %s\n", msg, remoteAddr)

		if msg.Type == message.MsgTypeRegister {
			fmt.Println("Peer connected:", remoteAddr)
			room := string(msg.Content)

			// Lock the peer list and add the new peer
			mu.Lock()
			// Add the new peer to the room
			newPeerConnection := addPeerToRoom(room, remoteAddr.String(), conn)

			// Send the list of existing peers to the new peer
			sendExistingPeersList(room, newPeerConnection, conn)
			mu.Unlock()
		}

		if msg.Type == message.MsgTypePeerHeartbeat {
			fmt.Printf("Received hearbeat from %s\n", remoteAddr)
			room := string(msg.Content)

			mu.Lock()
			recordHeartbeat(room, remoteAddr)
			mu.Unlock()
		}

		if msg.Type == message.MsgTypePeerDisconnected {
			fmt.Printf("Received disconnect from %s\n", remoteAddr)
			room := string(msg.Content)

			mu.Lock()
			removePeerFromRoom(room, remoteAddr.String(), conn)
			mu.Unlock()
		}
	}
}

// Remove stale connections based on last heartbeat
func pruneInactivePeers(conn *net.UDPConn) {
	for {
		time.Sleep(5 * time.Second)

		mu.Lock()
		for roomName, room := range rooms {
			for peerAddr, peer := range room.Peers {
				if time.Since(peer.LastHeartbeat) > peerTimeout {
					fmt.Printf("Peer %s in room %s timed out and disconnected.\n", peerAddr, roomName)
					removePeerFromRoom(roomName, peerAddr, conn)
				}
			}
		}
		mu.Unlock()
	}
}

// Record heartbeat from client
func recordHeartbeat(roomName string, peerAddr *net.UDPAddr) {
	room, roomExists := rooms[roomName]
	if roomExists {
		peer, peerExists := room.Peers[peerAddr.String()]
		if peerExists {
			peer.LastHeartbeat = time.Now()
		}
	}
}

// Add peer to list of peers
func addPeerToRoom(roomName string, peerAddr string, conn *net.UDPConn) *PeerConnection {
	room, exists := rooms[roomName]
	if !exists {
		room = &Room{
			Name:  roomName,
			Peers: make(map[string]*PeerConnection),
		}
		rooms[roomName] = room
	}
	r, _ := net.ResolveUDPAddr("udp", peerAddr)

	room.Peers[peerAddr] = &PeerConnection{
		Address:       peerAddr,
		LastHeartbeat: time.Now(),
		Conn:          r,
	}
	notifyRoomPeersAboutConnection(roomName, peerAddr, conn)

	return room.Peers[peerAddr]
}

func removePeerFromRoom(roomName, peerID string, conn *net.UDPConn) {
	room, exists := rooms[roomName]
	if !exists {
		return
	}
	delete(room.Peers, peerID)

	notifyRoomPeersAboutDisconnection(roomName, peerID, conn)
}

func notifyRoomPeersAboutConnection(roomName, message string, conn *net.UDPConn) {
	room, exists := rooms[roomName]
	if !exists {
		return
	}
	for _, peer := range room.Peers {
		if peer.Address != message {
			sendPeerConnectedMessage(peer.Conn, message, conn)
		}
	}
}

func notifyRoomPeersAboutDisconnection(roomName, message string, conn *net.UDPConn) {
	room, exists := rooms[roomName]
	if !exists {
		return
	}

	for _, peer := range room.Peers {
		sendPeerDisconnectedMessage(peer.Conn, message, conn)
	}
}

func sendExistingPeersList(roomName string, newPeerConnection *PeerConnection, conn *net.UDPConn) {
	room, exists := rooms[roomName]
	if !exists {
		return
	}

	for _, peer := range room.Peers {
		if peer.Address != newPeerConnection.Address {
			sendPeerConnectedMessage(newPeerConnection.Conn, peer.Address, conn)
		}
	}
}

func sendPeerConnectedMessage(addr net.Addr, messageContent string, conn *net.UDPConn) {
	peerConnectedMsg := &message.Message{
		Type:    message.MsgTypePeerConnected,
		Content: []byte(messageContent),
	}
	data, _ := peerConnectedMsg.Encode()

	_, err := conn.WriteTo(data, addr)
	if err != nil {
		fmt.Println("Error sending message to peer:", err)
	}
	fmt.Printf("Notified %s about peer %s\n connection", addr, []byte(messageContent))
}

func sendPeerDisconnectedMessage(addr net.Addr, messageContent string, conn *net.UDPConn) {
	peerConnectedMsg := &message.Message{
		Type:    message.MsgTypePeerDisconnected,
		Content: []byte(messageContent),
	}
	data, _ := peerConnectedMsg.Encode()

	_, err := conn.WriteTo(data, addr)
	if err != nil {
		fmt.Println("Error sending message to peer:", err)
	}
	fmt.Printf("Notified %s about peer disconnection %s\n", addr, []byte(messageContent))
}
