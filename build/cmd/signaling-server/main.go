package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"minivpn/internal/holepunch"
)

func main() {
	port := flag.Int("port", 51821, "UDP port to listen on")
	flag.Parse()

	fmt.Printf("Starting miniVPN Signaling Server on UDP port %d...\n", *port)

	server, err := holepunch.NewSignalingServer(*port)
	if err != nil {
		log.Fatalf("Failed to start signaling server: %v", err)
	}

	server.Start()
	fmt.Printf("Signaling server running on %s\n", server.GetAddr())
	fmt.Println("Press Ctrl+C to stop")

	// Wait for interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nShutting down...")
	server.Stop()
}
