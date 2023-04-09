package main

import (
	"bytes"
	"encoding/hex"
	"flag"
	"log"

	mbe "github.com/oesmith/can7/internal/mbe"
)

var ver_res = []byte{0xe4, 0x0, 0xd, 0x23, 0x39, 0x35, 0x39, 0x62, 0x64, 0x38, 0x30, 0x34, 0x0}

func main() {
	dev := flag.String("device", "vcan0", "Can device name")
	flag.Parse()

	con, err := mbe.NewConn(*dev, mbe.ID_EASIMAP, mbe.ID_ECU)
	if err != nil {
		log.Fatalf("Failed to open can device: %s\n", err)
	}
	defer con.Close()

	for {
		d, err := con.Recv()
		if err != nil {
			log.Fatalln(err)
		}
		if bytes.Equal(d, mbe.VersionReq) {
			log.Println("VER")
			if err := con.Send(ver_res); err != nil {
				log.Println(err)
			}
		} else if bytes.HasPrefix(d, mbe.DataReqPrefix) {
			pg := d[len(mbe.DataReqPrefix)]
			offs := d[len(mbe.DataReqPrefix)+1:]
			log.Printf("GET %x %s", pg, hex.EncodeToString(offs))
			res := make([]byte, len(offs)+len(mbe.DataResPrefix))
			copy(res, mbe.DataResPrefix)
			copy(res[len(mbe.DataResPrefix):], offs)
			if err := con.Send(res); err != nil {
				log.Println(err)
			}
		} else {
			log.Println("UNK:", hex.EncodeToString(d))
		}
	}
}
