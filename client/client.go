package client

import (
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/nmilo/gochat/message"
	"github.com/nmilo/gochat/p2pcrypto"
	"github.com/nmilo/gochat/ui"
)

var UI *ui.UI

type Peer struct {
	Address   string
	PublicKey string
	AesKey    []byte
}

var peers = make(map[string]*Peer)

var privKey *big.Int
var pubKey *big.Int

const (
	HeartbeatInterval = 10 // seconds
	LocalUDPPort      = ":4545"
)

// Start the client
func Start(bootnodeIP string, room string) {
	// Create a channel to receive input from the UI
	inputChan := make(chan string)
	peerMsgChan := make(chan string)

	// Set up Terminal UI
	UI = ui.NewUI(inputChan)
	UI.Run()

	// Generate inital pub/private key
	generateDHKeyPair()

	// Bootnode connection
	bootnodeAddr, err := net.ResolveUDPAddr("udp", bootnodeIP)
	if err != nil {
		fmt.Println("Error resolving bootnode address:", err)
		os.Exit(1)
	}

	// Local UDP connection for P2P communication and messages from Bootnode
	localAddr, _ := net.ResolveUDPAddr("udp", LocalUDPPort)
	localConn, _ := net.ListenUDP("udp", localAddr)

	// Register with the bootnode
	registerWithBootnode(room, localConn, bootnodeAddr)

	// Send heartbeat to Bootnode
	go sendHeartbeatToBootnode(room, localConn, bootnodeAddr)

	// Listen for UDP messages, either from Bootnode or Peers
	go listenForMessages(localConn, localAddr.String())

	// Set up a channel to listen for interrupts (Ctrl+C)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	// Wait for a signal
	go func() {
		sig := <-sigs
		fmt.Println("Received signal:", sig)

		// Perform graceful disconnect action
		disconnect(localConn, bootnodeAddr, room)

		fmt.Println("Graceful shutdown complete.")
		os.Exit(0)
	}()

	// Main loop to handle both user input and peer messages
	for {
		select {
		case input := <-inputChan:
			// User input received, send to peers
			if len(peers) > 0 {
				broadcastMessage(input, localConn)
			}

		case msg := <-peerMsgChan:
			// Message received from peers or bootnode
			fmt.Println(msg)
		}
	}
}

// Send register message to bootnode
func registerWithBootnode(room string, localConn *net.UDPConn, bootnodeAddr *net.UDPAddr) {
	registerMsg := &message.Message{
		Type:    message.MsgTypeRegister,
		Content: []byte(room),
	}
	data, _ := registerMsg.Encode()

	localConn.WriteTo(data, bootnodeAddr)
	UI.AppendContent("Sent Register to Bootnode")
}

// Broadcast message to all peers
func broadcastMessage(plaintextMessage string, conn *net.UDPConn) {
	for peerAddr, peer := range peers {
		ciphertext, err := p2pcrypto.EncryptMessage(peer.AesKey, plaintextMessage)
		if err != nil {
			fmt.Println("Error encrypting message: ", err)
			return
		}

		peerAddr, _ := net.ResolveUDPAddr("udp", peerAddr)
		chatMsg := &message.Message{
			Type:    message.MsgTypeChat,
			Content: ciphertext,
		}
		data, _ := chatMsg.Encode()

		conn.WriteTo(data, peerAddr)
	}
}

// Sent hearbeat to Bootnode every 10 seconds
func sendHeartbeatToBootnode(room string, conn *net.UDPConn, bootnodeAddr *net.UDPAddr) {
	ticker := time.NewTicker(HeartbeatInterval * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		heartbeatMsg := &message.Message{
			Type:    message.MsgTypePeerHeartbeat,
			Content: []byte(room),
		}
		data, _ := heartbeatMsg.Encode()

		conn.WriteTo(data, bootnodeAddr)
	}
}

// Listen for UDP messages
func listenForMessages(conn *net.UDPConn, local string) {
	for {
		buffer := make([]byte, 1024)

		n, remoteAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			fmt.Println("Error reading from UDP connection: ", err)
			continue
		}

		// Decode the message from bytes
		msg, err := message.Decode(buffer[:n])
		if err != nil {
			fmt.Println("Error decoding the message: ", err)
			continue
		}

		if msg.Type == message.MsgTypeChat {
			peer := peers[remoteAddr.String()]

			decryptedMessage, err := p2pcrypto.DecryptMessage(peer.AesKey, msg.Content)
			if err != nil {
				fmt.Println("Error decrypting the message: ", err)
				continue
			}

			UI.AppendContent(fmt.Sprintf("[yellow]%s[-]: %s", remoteAddr, decryptedMessage))
			continue
		}

		if msg.Type == message.MsgTypePeerConnected {
			peerAddr := string(msg.Content[:])

			peers[peerAddr] = &Peer{
				Address: peerAddr,
			}

			startKeyExchange(peers[peerAddr], conn)

			UI.AddUser(" " + peerAddr)
			UI.AppendContent(fmt.Sprintf("%s joined.", peerAddr))
		}

		if msg.Type == message.MsgTypePeerDisconnected {
			peerAddr := string(msg.Content[:])
			delete(peers, peerAddr)

			UI.RemoveUser(" " + peerAddr)
			UI.AppendContent(fmt.Sprintf("%s left.", peerAddr))
		}

		if msg.Type == message.MsgTypeKeyExchange {
			peer, peerExists := peers[remoteAddr.String()]
			if peerExists {
				peerPublicKey := string(msg.Content[:])
				peer.PublicKey = peerPublicKey

				n := new(big.Int)
				n, ok := n.SetString(peerPublicKey, 10)
				if !ok {
					fmt.Println("SetString: error")
					return
				}

				aesKey, err := p2pcrypto.PerformKeyExchange(privKey, n)
				if err != nil {
					log.Fatal(err)
				}
				peer.AesKey = aesKey
			}
		}

		for _, peer := range strings.Split(string(msg.Content[:]), ",") {
			if peer != local {
				go startHolePunching(conn, peer)
			}
		}
	}
}

// Generate inital key pair
func generateDHKeyPair() {
	privKey1, pubKey2, err := p2pcrypto.GenerateKeyPair()
	if err != nil {
		log.Fatal(err)
	}

	pubKey = pubKey2
	privKey = privKey1
}

// Send public key to peer for DH key exchange
func startKeyExchange(peer *Peer, conn *net.UDPConn) {
	peerAddr, _ := net.ResolveUDPAddr("udp", peer.Address)

	keyExchangeMsg := &message.Message{
		Type:    message.MsgTypeKeyExchange,
		Content: []byte(pubKey.String()),
	}
	data, _ := keyExchangeMsg.Encode()

	conn.WriteTo(data, peerAddr)
}

// Initialize NAT hole punching to keep P2P connection live
func startHolePunching(conn *net.UDPConn, peerIP string) {
	peerAddr, _ := net.ResolveUDPAddr("udp", peerIP)
	for {
		pingMsg := &message.Message{
			Type:    message.MsgTypePing,
			Content: []byte{},
		}
		data, _ := pingMsg.Encode()

		conn.WriteTo(data, peerAddr)
		time.Sleep(10 * time.Second)
	}
}

// Gracefull disconnect and notify bootnode
func disconnect(conn *net.UDPConn, bootnodeAddr *net.UDPAddr, room string) {
	disconnectMsg := &message.Message{
		Type:    message.MsgTypePeerDisconnected,
		Content: []byte(room),
	}
	data, _ := disconnectMsg.Encode()

	conn.WriteTo(data, bootnodeAddr)
}
