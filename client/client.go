package client

import (
	"context"
	"fmt"
	"math/big"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/nmilo/gochat/message"
	"github.com/nmilo/gochat/p2pcrypto"
	"github.com/nmilo/gochat/ui"
)

var UI *ui.UI

type Peer struct {
	Address                  string
	PublicKey                string
	AesKey                   []byte
	Cancel                   context.CancelFunc
	UDPConnectionEstablished bool
	KeysExchanged            bool
}

type Client struct {
	peers             map[string]*Peer
	privKey           *big.Int
	pubKey            *big.Int
	localUDPPort      string
	heartbeatInterval time.Duration
}

var localClient *Client

// Start the client
func Start(bootnodeIP string, room string, udpPort string) {
	// Create a channel to receive input from the UI
	inputChan := make(chan string)
	peerMsgChan := make(chan string)

	// Set up Terminal UI
	UI = ui.NewUI(inputChan)
	UI.Run()

	// Set up Client

	initializedClient, err := initClient(udpPort)
	if err != nil {
		UI.AppendContent(fmt.Sprintf("[red]error[-]: Error initializing the client: %s", err))
		os.Exit(1)
	}
	localClient = initializedClient

	// Bootnode connection
	bootnodeAddr, err := net.ResolveUDPAddr("udp", bootnodeIP)
	if err != nil {
		UI.AppendContent(fmt.Sprintf("Error resolving bootnode address: %s", err))
		os.Exit(1)
	}

	// Local UDP connection for P2P communication and messages from Bootnode
	localAddr, _ := net.ResolveUDPAddr("udp", localClient.localUDPPort)
	localConn, _ := net.ListenUDP("udp", localAddr)

	// Register with the bootnode
	registerWithBootnode(room, localConn, bootnodeAddr)

	// Send heartbeat to Bootnode
	go sendHeartbeatToBootnode(room, localConn, bootnodeAddr, localClient)

	// Listen for UDP messages, either from Bootnode or Peers
	go listenForMessages(localConn, localAddr.String())

	// Set up a channel to listen for interrupts (Ctrl+C)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	// Main loop to handle both user input and peer messages
	for {
		select {
		case input := <-inputChan:
			// User input received, send to peers
			if len(localClient.peers) > 0 {
				broadcastMessage(input, localConn)
			}

		case sig := <-sigs:
			fmt.Println("Received signal:", sig)

			// Perform graceful disconnect action
			disconnect(localConn, bootnodeAddr, room)

			fmt.Println("Graceful shutdown complete.")
			os.Exit(0)

		case msg := <-peerMsgChan:
			// Message received from peers or bootnode
			fmt.Println(msg)
		}
	}
}

// Initialize Client instance
func initClient(localUdpPort string) (*Client, error) {
	// Generate inital pub/private key
	privateKey, publicKey, err := generateDHKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate DH key pair: %v", err)
	}

	heartbeatInterval := 10 // seconds
	if os.Getenv("HEARTBEAT_INTERVAL") != "" {
		heartbeatInterval, err = strconv.Atoi(os.Getenv("HEARTBEAT_INTERVAL"))
		return nil, fmt.Errorf("failed to convert heartbeat interval to int: %v", err)
	}

	client := &Client{
		privKey:           privateKey,
		pubKey:            publicKey,
		localUDPPort:      localUdpPort,
		peers:             make(map[string]*Peer),
		heartbeatInterval: time.Duration(heartbeatInterval) * time.Second,
	}

	return client, nil
}

// Send register message to bootnode
func registerWithBootnode(room string, localConn *net.UDPConn, bootnodeAddr *net.UDPAddr) {
	UI.AppendContent("[blue]info[-]: Sent Register to Bootnode")

	defer localConn.SetReadDeadline(time.Time{})

	// Initialize retry variables
	retries := 0
	var ackReceived bool
	var maxRetries = 4                    // Maximum number of retry attempts
	var timeoutDuration = 2 * time.Second // Timeout for receiving confirmation

	// Build register message
	registerMsg := &message.Message{
		Type:    message.MsgTypeRegister,
		Content: []byte(room),
	}
	data, _ := registerMsg.Encode()

	// Retry loop for sending the registration message and waiting for acknowledgment
	for retries < maxRetries && !ackReceived {
		// Send registration message
		_, err := localConn.WriteTo(data, bootnodeAddr)

		if err != nil {
			UI.AppendContent(fmt.Sprintf("Error sending registration message: %s", err))
			return
		}

		if retries > 0 {
			UI.AppendContent(fmt.Sprintf("[blue]info[-]: Retrying to connect with the bootnode... Attempt %d/%d", retries, maxRetries-1))
		}

		// Set a read deadline for the confirmation
		localConn.SetReadDeadline(time.Now().Add(timeoutDuration))

		// Buffer to store incoming response
		buffer := make([]byte, 1024)

		// Wait for acknowledgment from the bootnode
		n, _, err := localConn.ReadFromUDP(buffer)
		if err == nil {
			// Extract and process the acknowledgment message
			msg, err := message.Decode(buffer[:n])
			if err != nil {
				UI.AppendContent(fmt.Sprintf("Error decoding the message: %s", err))
				continue
			}

			confirmationMessage := string(msg.Content)

			// Check if the acknowledgment matches the expected "ACK"
			if confirmationMessage == "ACK" {
				UI.AppendContent("[blue]info[-]: Connection established with Bootnode")

				ackReceived = true
			} else {
				UI.AppendContent(fmt.Sprintf("Unexpected response from bootnode: %s", confirmationMessage))
			}
		}

		// Increment retry count if acknowledgment is not received
		if !ackReceived {
			retries++

			if retries < maxRetries {
				// Add a short delay before retrying
				time.Sleep(2 * time.Second)
			}
		}
	}

	// Exit if no acknowledgment was recieved from bootnode after max retry attempts
	if !ackReceived {
		UI.AppendContent(fmt.Sprintf("[blue]info[-]: Failed to register with the bootnode after %d attempts. Exiting.\n", maxRetries))
		os.Exit(1)
	}
}

// Broadcast message to all peers
func broadcastMessage(plaintextMessage string, conn *net.UDPConn) {
	for peerAddr, peer := range localClient.peers {

		if len(peer.AesKey) == 0 {
			UI.AppendContent("[blue]info[-]: Unable to send message, key exchange not completed.")
			continue
		}

		ciphertext, err := p2pcrypto.EncryptMessage(peer.AesKey, plaintextMessage)
		if err != nil {
			UI.AppendContent(fmt.Sprintf("Error encrypting the message: %s", err))
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
func sendHeartbeatToBootnode(room string, conn *net.UDPConn, bootnodeAddr *net.UDPAddr, client *Client) {
	heartbeatMsg := &message.Message{
		Type:    message.MsgTypePeerHeartbeat,
		Content: []byte(room),
	}
	data, _ := heartbeatMsg.Encode()

	for {
		conn.WriteTo(data, bootnodeAddr)
		time.Sleep(client.heartbeatInterval)
	}
}

// Listen for UDP messages
func listenForMessages(conn *net.UDPConn, local string) {
	for {
		buffer := make([]byte, 1024)

		n, remoteAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			UI.AppendContent(fmt.Sprintf("Error reading from UDP connection: %s", err))
			continue
		}

		// Decode the message from bytes
		msg, err := message.Decode(buffer[:n])
		if err != nil {
			UI.AppendContent(fmt.Sprintf("Error decoding the message: %s", err))
			continue
		}

		if msg.Type == message.MsgTypeChat {
			peer := localClient.peers[remoteAddr.String()]

			decryptedMessage, err := p2pcrypto.DecryptMessage(peer.AesKey, msg.Content)
			if err != nil {
				UI.AppendContent(fmt.Sprintf("Error decrypting the message: %s", err))
				continue
			}

			UI.AppendContent(fmt.Sprintf("[yellow]%s[-]: %s", remoteAddr, decryptedMessage))
			continue
		}

		if msg.Type == message.MsgTypePeerConnected {
			ctx, cancel := context.WithCancel(context.Background())

			peerAddr := string(msg.Content[:])
			peer := &Peer{
				Address:                  peerAddr,
				UDPConnectionEstablished: false,
				KeysExchanged:            false,
				Cancel:                   cancel,
			}

			localClient.peers[peerAddr] = peer

			// Spark UDP connection
			go sparkConnection(ctx, conn, peer)

			// Update TUI
			UI.AddUser(" " + peerAddr)
			UI.AppendContent(fmt.Sprintf("[blue]info[-]: %s joined.", peerAddr))
		}

		if msg.Type == message.MsgTypePeerDisconnected {
			peerAddr := string(msg.Content[:])

			peer := localClient.peers[peerAddr]
			peer.Cancel()
			delete(localClient.peers, peerAddr)

			UI.RemoveUser(" " + peerAddr)
			UI.AppendContent(fmt.Sprintf("[blue]info[-]: %s left.", peerAddr))
		}

		if msg.Type == message.MsgTypePing {
			peer, peerExists := localClient.peers[remoteAddr.String()]

			if peerExists && !peer.UDPConnectionEstablished {
				// UDP tunnel established
				peer.UDPConnectionEstablished = true

				// Perform DH key exchange
				go startKeyExchange(peer, conn)
			}
		}

		if msg.Type == message.MsgTypeKeyExchange {
			peer, peerExists := localClient.peers[remoteAddr.String()]
			if peerExists {
				peerPublicKey := string(msg.Content[:])
				peer.PublicKey = peerPublicKey

				n := new(big.Int)
				n, ok := n.SetString(peerPublicKey, 10)
				if !ok {
					UI.AppendContent(fmt.Sprintf("Error parsing peer's public key: %s", peerPublicKey))
					continue
				}

				aesKey, err := p2pcrypto.PerformKeyExchange(localClient.privKey, n)
				if err != nil {
					UI.AppendContent(fmt.Sprintf("Error performing key exchange: %s", err))
					continue
				}

				peer.AesKey = aesKey
				peer.KeysExchanged = true
			}
		}
	}
}

// Generate inital key pair
func generateDHKeyPair() (*big.Int, *big.Int, error) {
	privKey, pubKey, err := p2pcrypto.GenerateKeyPair()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate DH key pair: %v", err)
	}

	return privKey, pubKey, nil
}

// Send public key to peer for DH key exchange
func startKeyExchange(peer *Peer, conn *net.UDPConn) {
	peerAddr, _ := net.ResolveUDPAddr("udp", peer.Address)

	keyExchangeMsg := &message.Message{
		Type:    message.MsgTypeKeyExchange,
		Content: []byte(localClient.pubKey.String()),
	}
	keyExchangeData, _ := keyExchangeMsg.Encode()

	conn.WriteTo(keyExchangeData, peerAddr)
}

// Send UDP packets to establish NAT hole (UDP connection)
func sparkConnection(ctx context.Context, conn *net.UDPConn, peer *Peer) {
	peerAddr, _ := net.ResolveUDPAddr("udp", peer.Address)
	pingMsg := &message.Message{
		Type:    message.MsgTypePing,
		Content: []byte{},
	}
	data, _ := pingMsg.Encode()

	// Initialize retry variables
	retries := 0
	var maxRetries = 10 // Maximum number of retry attempts

	for retries < maxRetries {
		retries++

		if peer.UDPConnectionEstablished {
			// Start new goroutine to maintain UDP connection
			go maintainUDPConnection(ctx, conn, peer)
			break
		}

		// Send UDP packet every 200 ms
		conn.WriteTo(data, peerAddr)
		time.Sleep(200 * time.Millisecond)
	}

	if !peer.UDPConnectionEstablished {
		UI.AppendContent(fmt.Sprintf("[red]error[-]: Failed to establish UDP connection with peer: %s", peer.Address))
	}
}

// Periodically send UDP packet to keep NAT record active on peer's machine and P2P connection live
func maintainUDPConnection(ctx context.Context, conn *net.UDPConn, peer *Peer) {
	peerAddr, _ := net.ResolveUDPAddr("udp", peer.Address)
	pingMsg := &message.Message{
		Type:    message.MsgTypePing,
		Content: []byte{},
	}
	data, _ := pingMsg.Encode()

	for {
		select {
		case <-ctx.Done():
			// Context was canceled, exit the goroutine
			return
		default:
			// Send Ping every 10 seconds
			conn.WriteTo(data, peerAddr)
			time.Sleep(10 * time.Second)
		}
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
