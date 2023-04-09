// Replay a CAN packet capture from wireshark / tshark.
package main

import (
	"context"
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/google/gopacket/pcapgo"
	"go.einride.tech/can"
	"go.einride.tech/can/pkg/socketcan"
)

type CapFrame struct {
	_ [16]byte
	IDFlags uint32
	Size uint8
	_ [3]byte
	Data [8]byte
}

func main() {
	capFile := flag.String("capfile", "dump.pcapng", "Capture file name")
	canDev := flag.String("interface", "vcan0", "Can interface name (e.g., vcan0)")
	flag.Parse()

	f, err := os.Open(*capFile)
	if err != nil {
		log.Fatalf("Unable to open capture file: %s\n", err)
	}
	defer f.Close()

	r, err := pcapgo.NewNgReader(f, pcapgo.DefaultNgReaderOptions)
	if err != nil {
		log.Fatalf("Unable to read pcapng file: %s\n", err)
	}

	con, err := socketcan.DialContext(context.Background(), "can", *canDev)
	if err != nil {
		log.Fatalf("Failed to open can device: %s\n", err)
	}
	tx := socketcan.NewTransmitter(con)

	var lastTs time.Time

	for {
		data, ci, err := r.ReadPacketData()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Failed to read packet: %s\n", err)
		}

		// Naive attempt to match the timing of the original packet capture.
		if !lastTs.IsZero() {
			dt := ci.Timestamp.Sub(lastTs)
			time.Sleep(dt)
		}
		lastTs = ci.Timestamp

		// Parse the CAN packet header from the pcap file, which seems to have its
		// own structure.
		var f CapFrame
		buf := bytes.NewReader(data)
		err = binary.Read(buf, binary.LittleEndian, &f)

		// Log the frame.
		var hex bytes.Buffer
		for _, b := range f.Data {
			hex.WriteString(fmt.Sprintf("%02x ", b))
		}
		id := f.IDFlags & 0x1fffffff
		log.Printf("%s %8x %s", ci.Timestamp, id, hex.String())

		// Transmit the frame on CAN.
		txf := can.Frame{
			ID: id,
			Length: f.Size,
			Data: f.Data,
			IsRemote: f.IDFlags & 0x40000000 > 0,
			IsExtended: f.IDFlags & 0x80000000 > 0,
		}
		if err := tx.TransmitFrame(context.Background(), txf); err != nil {
			log.Printf("Error sending packet: %s\n", err)
		}
	}
}
