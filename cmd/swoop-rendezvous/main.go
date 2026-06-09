// swoop-rendezvous is a minimal signaling server for internet P2P pairing.
// It relays overlay bytes only while an active invite session exists (no file store).
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
