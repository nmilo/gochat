package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/nmilo/gochat/bootnode"
	"github.com/nmilo/gochat/client"
)

func main() {
	godotenv.Load(".env")

	defaultUdpPort := "4545" // Default value
	if os.Getenv("UDP_PORT") != "" {
		defaultUdpPort = os.Getenv("UDP_PORT")
	}

	// Set command-line flags
	mode := flag.String("mode", "", "Mode to run (bootnode or client)")

	// Bootnode mode flags
	listen := flag.String("listen", "0.0.0.0:9595", "Listen address")

	// Client mode flags
	room := flag.String("room", "", "Room identifier")
	bootnodeIP := flag.String("bootnode", "", "Bootnode IP address for client mode")
	udpPort := flag.String("port", defaultUdpPort, "Local UDP port")

	// Parse the command-line flags
	flag.Parse()

	// Validate mode
	if *mode != "bootnode" && *mode != "client" {
		fmt.Println("Error: Mode must be 'bootnode' or 'client'")
		os.Exit(1)
	}

	// Validate args for client
	if *mode == "client" && *room == "" {
		fmt.Println("Please provide a room.")
		os.Exit(1)
	}

	// Validate args for bootnode
	if *mode == "bootnode" && *listen == "" {
		fmt.Println("Please provide a listening address:port.")
		os.Exit(1)
	}

	switch *mode {
	case "bootnode":
		bootnode.Start(*listen)
	case "client":
		client.Start(*bootnodeIP, *room, *udpPort)
	default:
		fmt.Println("Invalid mode. Use 'bootnode' or 'client'.")
		os.Exit(1)
	}
}
