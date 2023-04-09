package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/oesmith/can7/internal/mbe"
)

type model struct {
	con mbe.Conn

	ver version

	state ecuParams
}

type version struct {
	ver string
	err error
}

type ecuParams struct {
	rpm uint16  // Engine speed.
	tps float32 // TPS site.
	bat float32 // Battery voltage.
	ct  float32 // Coolant temp.
	at  float32 // Air temp.
	ot  float32 // Oil temp.
	err error
}

type other struct{}

func main() {
	dev := flag.String("device", "vcan0", "Can device name")
	flag.Parse()

	con, err := mbe.NewConn(*dev, mbe.ID_ECU, mbe.ID_EASIMAP)
	if err != nil {
		log.Fatalf("Failed to open can device: %s\n", err)
	}
	defer con.Close()

	if _, err := tea.NewProgram(model{con: con}).Run(); err != nil {
		log.Fatalln("Error running program", err)
	}
}

func (m model) Init() tea.Cmd {
	return identify(m.con)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
	case version:
		m.ver = msg
		return m, poll(m.con)
	case ecuParams:
		m.state = msg
		return m, poll(m.con)
	}
	return m, nil
}

func (m model) View() string {
	s := "MBE Scan\n"
	if m.ver.err != nil {
		s += fmt.Sprintf("! Version fetch failed: %s\n", m.ver.err)
	} else if m.ver.ver != "" {
		s += fmt.Sprintf("  Version: %s\n", m.ver.ver)
	}
	if m.state.err != nil {
		s += fmt.Sprintf("! Data fetch failed: %s\n", m.state.err)
	} else {
		s += fmt.Sprintf("  Battery:      %4.1f V\n", m.state.bat)
		s += fmt.Sprintf("  Engine speed: %4d rpm\n", m.state.rpm)
		s += fmt.Sprintf("  TPS site:     %4.1f\n", m.state.tps)
		s += fmt.Sprintf("  Air temp:     %4.1f deg C\n", m.state.at)
		s += fmt.Sprintf("  Water temp:   %4.1f deg C\n", m.state.ct)
		s += fmt.Sprintf("  Oil temp:     %4.1f deg C\n", m.state.ot)
	}
	return s
}

func identify(con mbe.Conn) tea.Cmd {
	return func() tea.Msg {
		if err := con.Send(mbe.VersionReq); err != nil {
			return version{err: err}
		}
		res, err := con.Recv()
		if err != nil {
			return version{err: err}
		}
		ver, err := mbe.ParseVersionResponse(res)
		if err != nil {
			return version{err: err}
		}
		return version{ver: ver}
	}
}

func poll(con mbe.Conn) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(50 * time.Millisecond)
		s := ecuParams{}

		reqs := [][]byte{
			mbe.CreateDataRequest(0xf8, []byte{
				0x7d, 0x7d, // RT_ENGINESPEED
				0x9f, 0x9e, // RT_BATTERYVOLTAGE(LIM)
				0x64,       // RT_THROTTLESITE1
				0x45, 0x44, // RT_COOLANTTEMP1(LIM)
				0x37, 0x36, // RT_AIRTEMP1(LIM)
				0x47, 0x46, // RT_OILTEMP1(LIM)
			}),
		}
		res := make([][]byte, len(reqs))
		for i, req := range reqs {
			if err := con.Send(req); err != nil {
				return ecuParams{err: err}
			}
			r, err := con.Recv()
			if err != nil {
				return ecuParams{err: err}
			}
			res[i], err = mbe.ParseDataResponse(r)
			if err != nil {
				return ecuParams{err: err}
			}
		}

		s.rpm = uint16(res[0][0])<<8 + uint16(res[0][1])
		s.bat = float32(uint16(res[0][2])<<8+uint16(res[0][3])) * 20.0 / 65535.0
		s.tps = float32(res[0][4]) * 16.0 / 255.0
		s.ct = float32(uint16(res[0][5])<<8+uint16(res[0][3]))*160.0/65535.0 - 30.0
		s.at = float32(uint16(res[0][7])<<8+uint16(res[0][8]))*160.0/65535.0 - 30.0
		s.ot = float32(uint16(res[0][9])<<8+uint16(res[0][10]))*160.0/65535.0 - 30.0

		return s
	}
}
