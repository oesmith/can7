package mbe

import (
	"context"
	"fmt"
	"net"
	"time"

	"go.einride.tech/can"
	"go.einride.tech/can/pkg/socketcan"
)

type Conn interface {
	Close() error
	Recv() (msg []byte, err error)
	Reopen() (error, Conn)
	Send(msg []byte) error
	SetTimeout(t time.Duration)
}

type connImpl struct {
	dev   string
	rx_id uint32     // Identity to receive CAN frames from.
	tx_id uint32     // Identity to send CAN frames as.
	con   net.Conn
	rx    *socketcan.Receiver
	tx    *socketcan.Transmitter
}

func NewConn(dev string, rx_id uint32, tx_id uint32) (Conn, error) {
	c := connImpl{
		dev: dev,
		rx_id: rx_id,
		tx_id: tx_id,
	}
	if err := c.open(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *connImpl) open() error {
	con, err := socketcan.DialContext(context.Background(), "can", c.dev)
	if err != nil {
		return fmt.Errorf("Failed to open can device: %s\n", err)
	}
	c.con = con
	c.rx = socketcan.NewReceiver(con)
	c.tx = socketcan.NewTransmitter(con)
	return nil
}

func (c connImpl) Reopen() (error, Conn) {
	c.Close()
	if err := c.open(); err != nil {
		return err, nil
	}
	return nil, c
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

func (c connImpl) SetTimeout(t time.Duration) {
	c.con.SetDeadline(time.Now().Add(t))
}
