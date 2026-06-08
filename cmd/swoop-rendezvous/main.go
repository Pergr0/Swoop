// swoop-rendezvous is a minimal signaling server for internet P2P pairing.
// It never relays file data — only helps peers discover endpoints for hole punching.
//
// Build (Linux): scripts/build-server.sh
package main

import (
	"flag"
	"log"

	"swoop/core/rendezvous/server"
)

func main() {
	addr := flag.String("addr", ":53400", "listen address")
	flag.Parse()
	s := server.New(*addr)
	if err := s.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
