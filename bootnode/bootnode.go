package bootnode

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
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

type Bootnode struct {
	mu              sync.Mutex
	rooms           map[string]*Room
	peerTimeout     time.Duration
	ecdsaPublicKey  *ecdsa.PublicKey
	ecdsaPrivateKey *ecdsa.PrivateKey
}

var localBootnode *Bootnode

func Start(listen string) {
	// Initialize bootnode
	initializedBootnode, err := initializeBootnode()
	if err != nil {
		fmt.Println("Error initializing bootnode:", err)
		os.Exit(1)
	}
	localBootnode = initializedBootnode

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
			peerAddr := remoteAddr.String()

			// Lock the peer list and add the new peer
			localBootnode.mu.Lock()

			// Add the new peer to the room
			newPeerConnection := addPeerToRoom(room, peerAddr, conn)

			// Unlock the peer list
			localBootnode.mu.Unlock()

			// Send Register Success back to client
			sendRegisterSucccess(newPeerConnection, conn, msg.ExtraContent)

			// Notify room peers about new connection
			notifyRoomPeersAboutConnection(room, peerAddr, conn)

			// Send the list of existing peers to the new peer
			sendExistingPeersList(room, newPeerConnection, conn)
		}

		if msg.Type == message.MsgTypePeerHeartbeat {
			fmt.Printf("Received hearbeat from %s\n", remoteAddr)
			room := string(msg.Content)

			localBootnode.mu.Lock()
			recordHeartbeat(room, remoteAddr)
			localBootnode.mu.Unlock()
		}

		if msg.Type == message.MsgTypePeerDisconnected {
			fmt.Printf("Received disconnect from %s\n", remoteAddr)
			room := string(msg.Content)

			localBootnode.mu.Lock()
			removePeerFromRoom(room, remoteAddr.String(), conn)
			localBootnode.mu.Unlock()
		}
	}
}

// Initialize bootnode struct
func initializeBootnode() (*Bootnode, error) {
	peerTimeout := 15 // seconds
	if os.Getenv("PEER_TIMEOUT") != "" {
		peerTimeout, _ = strconv.Atoi(os.Getenv("PEER_TIMEOUT"))
	}

	// Generate the server's ECDSA private and public keys
	serverPriv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatal(err)
	}
	serverPub := serverPriv.PublicKey

	bootnode := &Bootnode{
		rooms:           make(map[string]*Room),
		mu:              sync.Mutex{},
		peerTimeout:     time.Duration(peerTimeout) * time.Second,
		ecdsaPublicKey:  &serverPub,
		ecdsaPrivateKey: serverPriv,
	}

	return bootnode, nil
}

// Remove stale connections based on last heartbeat
func pruneInactivePeers(conn *net.UDPConn) {
	for {
		time.Sleep(5 * time.Second)

		localBootnode.mu.Lock()
		for roomName, room := range localBootnode.rooms {
			for peerAddr, peer := range room.Peers {
				if time.Since(peer.LastHeartbeat) > localBootnode.peerTimeout {
					fmt.Printf("Peer %s in room %s timed out and disconnected.\n", peerAddr, roomName)
					removePeerFromRoom(roomName, peerAddr, conn)
				}
			}
		}
		localBootnode.mu.Unlock()
	}
}

// Send back register success to the client
func sendRegisterSucccess(peerConnection *PeerConnection, conn *net.UDPConn, clientPubBytes []byte) {
	// Marshall bootnode's public key
	pubBytes, err := x509.MarshalPKIXPublicKey(&localBootnode.ecdsaPublicKey)
	if err != nil {
		fmt.Println("Error marshalling bootnode public key:", err)
		return
	}

	serverPriv := localBootnode.ecdsaPrivateKey
	clientPubKey, err := x509.ParsePKIXPublicKey(clientPubBytes)
	if err != nil {
		log.Println("Failed to parse client's public key:", err)
		return
	}
	clientPub := clientPubKey.(*ecdsa.PublicKey)

	sharedSecretX, _ := serverPriv.PublicKey.ScalarMult(clientPub.X, clientPub.Y, serverPriv.D.Bytes())
	sharedSecret := sha256.Sum256(sharedSecretX.Bytes())
	fmt.Printf("Server shared secret: %x\n", sharedSecret)

	// Sign the shared secret with the server's private key
	r, s, err := ecdsa.Sign(rand.Reader, serverPriv, sharedSecret[:])
	if err != nil {
		log.Println("Failed to sign shared secret:", err)
		return
	}
	sig := append(r.Bytes(), s.Bytes()...)

	// Build Register Success message for client
	registerSuccessMsg := &message.Message{
		Type:         message.MsgTypeRegister,
		Content:      pubBytes,
		ExtraContent: sig,
	}
	data, _ := registerSuccessMsg.Encode()

	_, err = conn.WriteTo(data, peerConnection.Conn)
	if err != nil {
		fmt.Println("Error sending message to peer:", err)
		return
	}

	fmt.Printf("Sent Register Success to %s\n", peerConnection.Conn.String())
}

// Record heartbeat from client
func recordHeartbeat(roomName string, peerAddr *net.UDPAddr) {
	room, roomExists := localBootnode.rooms[roomName]
	if roomExists {
		peer, peerExists := room.Peers[peerAddr.String()]
		if peerExists {
			peer.LastHeartbeat = time.Now()
		}
	}
}

// Add peer to list of peers
func addPeerToRoom(roomName string, peerAddr string, conn *net.UDPConn) *PeerConnection {
	room, exists := localBootnode.rooms[roomName]
	if !exists {
		room = &Room{
			Name:  roomName,
			Peers: make(map[string]*PeerConnection),
		}
		localBootnode.rooms[roomName] = room
	}
	r, _ := net.ResolveUDPAddr("udp", peerAddr)

	room.Peers[peerAddr] = &PeerConnection{
		Address:       peerAddr,
		LastHeartbeat: time.Now(),
		Conn:          r,
	}

	return room.Peers[peerAddr]
}

func removePeerFromRoom(roomName, peerID string, conn *net.UDPConn) {
	room, exists := localBootnode.rooms[roomName]
	if !exists {
		return
	}
	delete(room.Peers, peerID)

	notifyRoomPeersAboutDisconnection(roomName, peerID, conn)
}

func notifyRoomPeersAboutConnection(roomName, message string, conn *net.UDPConn) {
	room, exists := localBootnode.rooms[roomName]
	if !exists {
		return
	}
	for _, peer := range room.Peers {
		if peer.Address != message {
			sendPeerConnectedMessage(peer.Conn, message, conn)
		}
	}
}

// Sends message to peers about peer disconnection
func notifyRoomPeersAboutDisconnection(roomName, peerID string, conn *net.UDPConn) {
	room, exists := localBootnode.rooms[roomName]
	if !exists {
		return
	}

	for _, peer := range room.Peers {
		sendPeerDisconnectedMessage(peer.Conn, peerID, conn)
	}
}

// Sends list of existing peers one by one
func sendExistingPeersList(roomName string, newPeerConnection *PeerConnection, conn *net.UDPConn) {
	room, exists := localBootnode.rooms[roomName]
	if !exists {
		return
	}

	for _, peer := range room.Peers {
		if peer.Address != newPeerConnection.Address {
			sendPeerConnectedMessage(newPeerConnection.Conn, peer.Address, conn)
		}
	}
}

// Sends message about peer connection
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

// Sends message about peer disconnection
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
