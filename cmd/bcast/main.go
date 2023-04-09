package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"go.einride.tech/can/pkg/socketcan"
	tea "github.com/charmbracelet/bubbletea"
)

const BROADCAST_ID = 0xCBB0001

type model struct {
	rx *socketcan.Receiver // Can receiver
	p1 page1
	p2 page2
	p4 page4
}

type page1 struct {
	ct float32 // Coolant temperature.
	rpm int32 // Engine speed.
	cel float32 // Calculated engine load.
	tp float32 // Throttle position.
	bat1 float32 // Battery voltage (1).
	iat float32 // Intake air temperature.
}

type page2 struct {
	mp float32 // Manifolt air pressure.
}

type page4 struct {
	bat2 float32 // Battery voltage (2).
}

type other struct {}

func main() {
	dev := flag.String("device", "vcan0", "Can device name")
	flag.Parse()

	con, err := socketcan.DialContext(context.Background(), "can", *dev)
	if err != nil {
		log.Fatalf("Failed to open can device: %s\n", err)
	}

	rx := socketcan.NewReceiver(con)
	defer rx.Close()

	if _, err := tea.NewProgram(model{rx: rx}).Run(); err != nil {
		log.Fatalln("Error running program", err)
	}
}

func (m model) Init() tea.Cmd {
	return recv_can(m.rx)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
	case page1:
		m.p1 = msg
		return m, recv_can(m.rx)
	case page2:
		m.p2 = msg
		return m, recv_can(m.rx)
	case page4:
		m.p4 = msg
		return m, recv_can(m.rx)
	case other:
		return m, recv_can(m.rx)
	}
	return m, nil
}

func (m model) View() string {
	s := fmt.Sprintf("MBE Broadcast data\n\n")
	s += fmt.Sprintf("  Coolant temperature:    %3.1f deg C\n", m.p1.ct)
	s += fmt.Sprintf("  Engine speed:          %d rpm\n", m.p1.rpm)
	s += fmt.Sprintf("  Calculated engine load: %3.0f %%\n", m.p1.cel)
	s += fmt.Sprintf("  Throttle position:      %3.0f %%\n", m.p1.tp)
	s += fmt.Sprintf("  Battery voltage (1):     %2.1f V\n", m.p1.bat1)
	s += fmt.Sprintf("  Intake air temperature: %3.1f deg C\n", m.p1.iat)
	s += fmt.Sprintf("  Manifold air pressure:  %3.0f kPA\n", m.p2.mp)
	s += fmt.Sprintf("  Battery voltage (2):     %2.1f V\n", m.p4.bat2)
	return s
}

func recv_can(rx *socketcan.Receiver) tea.Cmd {
	return func() tea.Msg {
		if !rx.Receive() {
			return tea.Quit
		}
		frame := rx.Frame()
		if frame.ID == BROADCAST_ID {
			page := frame.Data[0]
			switch page {
			case 1:
				return page1{
					ct: float32(frame.Data[1]) * 160.0 / 255.0 - 30.0,
					rpm: int32(frame.Data[2]) + int32(frame.Data[3]) * 256,
					cel: float32(frame.Data[4]) * 100.0 / 255.0,
					tp: float32(frame.Data[5]) * 100.0 / 255.0,
					bat1: float32(frame.Data[6]) * 16.0 / 255.0 + 2.5,
					iat: float32(frame.Data[7]) * 160.0 / 255.0 - 30.0,
				}
			case 2:
				return page2{
					mp: float32(frame.Data[5]) * 122.0 / 255.0,
				}
			case 4:
				return page4{
					bat2: float32(frame.Data[1]) * 16.0 / 255.0 + 2.5,
				}
			default:
			}
		}
		return other{}
	}
}
