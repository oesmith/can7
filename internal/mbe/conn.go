package mbe

import (
	"context"
	"fmt"
	"net"

	"go.einride.tech/can"
	"go.einride.tech/can/pkg/socketcan"
)

type Conn interface {
	Close() error
	Recv() (msg []byte, err error)
	Send(msg []byte) error
}

type connImpl struct {
	con   net.Conn
	rx    *socketcan.Receiver
	tx    *socketcan.Transmitter
	rx_id uint32     // Identity to receive CAN frames from.
	tx_id uint32     // Identity to send CAN frames as.
}

func NewConn(dev string, rx_id uint32, tx_id uint32) (Conn, error) {
	con, err := socketcan.DialContext(context.Background(), "can", dev)
	if err != nil {
		return nil, fmt.Errorf("Failed to open can device: %s\n", err)
	}
	c := connImpl{
		con:   con,
		rx:    socketcan.NewReceiver(con),
		tx:    socketcan.NewTransmitter(con),
		rx_id: rx_id,
		tx_id: tx_id,
	}
	return c, nil
}

func (c connImpl) Close() error {
	if err := c.tx.Close(); err != nil {
		return err
	}
	if err := c.rx.Close(); err != nil {
		return err
	}
	if err := c.con.Close(); err != nil {
		return err
	}
	return nil
}

// TODO: handle timeouts.
func (c connImpl) Recv() ([]byte, error) {
	state := recv{}
	for {
		if !c.rx.Receive() {
			return nil, c.rx.Err()
		}

		f := c.rx.Frame()
		if f.ID != c.rx_id {
			continue
		}

		if err := state.update(f.Data); err != nil {
			return nil, err
		}

		if state.isComplete() {
			break
		}
	}
	return state.bytes(), nil
}

// TODO: pass context properly.
func (c connImpl) Send(msg []byte) error {
	if len(msg) == 0 || len(msg) > 4095 {
		return fmt.Errorf("Invalid message size: %d", len(msg))
	}
	if len(msg) <= 7 {
		// Send in a single frame.
		f := can.Frame{ID: c.tx_id, IsExtended: true, Length: uint8(len(msg) + 1)}
		f.Data[0] = byte(len(msg))
		copy(f.Data[1:], msg)
		return c.tx.TransmitFrame(context.Background(), f)
	}
	// First frame.
	f := can.Frame{ID: c.tx_id, IsExtended: true, Length: 8}
	f.Data[0] = 0x10 + byte(len(msg) >> 8)
	f.Data[1] = byte(len(msg) & 0xff)
	n_sent := copy(f.Data[2:], msg[:6])
	if err := c.tx.TransmitFrame(context.Background(), f); err != nil {
		return err
	}
	// Subsequent frames.
	idx := 1
	for {
		f.Data[0] = 0x20 + byte(idx) & 0xf
		n_sent += copy(f.Data[1:8], msg[n_sent:])
		if err := c.tx.TransmitFrame(context.Background(), f); err != nil {
			return err
		}
		if n_sent >= len(msg) {
			break
		}
		idx += 1
	}
	return nil
}
