package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/oesmith/can7/internal/mbe"
)

func main() {
	dev := flag.String("device", "vcan0", "Can device name")
	flag.Parse()

	con, err := mbe.NewConn(*dev, mbe.ID_ECU, mbe.ID_EASIMAP)
	if err != nil {
		log.Fatalf("Failed to open can device: %s\n", err)
	}
	defer con.Close()

	con.Send(mbe.VersionReq)
	r, err := con.Recv()
	if err != nil {
		log.Fatalln(err)
	}

	ver, err := mbe.ParseVersionResponse(r)
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println("ECU version", ver)
}
