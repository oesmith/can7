package main

import (
	"flag"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/oesmith/can7/internal/mbe"
)

var title = lipgloss.NewStyle().
	Bold(true).
	BorderStyle(lipgloss.DoubleBorder()).
	Padding(1).
	Width(50).
	AlignHorizontal(lipgloss.Center)

type model struct {
	con mbe.Conn

	params []mbe.Param
	pages []page

	ver version

	state ecuParams
}

type version struct {
	ver string
	err error
}

type ecuParams struct {
	vals map[string]string
	err  error
}

type page struct {
	pg byte
	addrs []byte
}

type other struct{}

func main() {
	dev := flag.String("device", "can0", "Can device name")
	config := flag.String("config", "params.yaml", "Params file name")
	flag.Parse()

	con, err := mbe.NewConn(*dev, mbe.ID_ECU, mbe.ID_EASIMAP)
	if err != nil {
		log.Fatalf("Failed to open can device: %s\n", err)
	}
	defer con.Close()

	params, err := mbe.LoadParams(*config)
	if err != nil {
		log.Fatalf("Failed to load config: %s\n", err)
	}

	if _, err := tea.NewProgram(model{con: con, params: params, pages: pages(params)}, tea.WithAltScreen()).Run(); err != nil {
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
		return m, poll(m.con, m.pages, m.params)
	case ecuParams:
		m.state = msg
		return m, poll(m.con, m.pages, m.params)
	}
	return m, nil
}

func (m model) View() string {
	s := title.Render("Caterscan") + "\n"
	if m.ver.err != nil {
		s += fmt.Sprintf("! Version fetch failed: %s\n", m.ver.err)
	} else if m.ver.ver != "" {
		s += fmt.Sprintf("%25s  %s\n", "Serial", m.ver.ver)
	}
	if m.state.err != nil {
		s += fmt.Sprintf("! Data fetch failed: %s\n", m.state.err)
	} else {
		for _, param := range m.params {
			val, ok := m.state.vals[param.ID]
			if ok {
				s += fmt.Sprintf("%25s  %s\n", param.Name, val)
			} else {
				s += fmt.Sprintf("%25s  nil\n", param.Name)
			}
		}
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

func poll(con mbe.Conn, pages []page, params []mbe.Param) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(50 * time.Millisecond)

		reqs := make([][]byte, len(pages))
		for n, p := range pages {
			reqs[n] = mbe.CreateDataRequest(p.pg, p.addrs)
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

		d := map[uint16]byte{}
		for i, p := range pages {
			for j, o := range p.addrs {
				d[uint16(p.pg) << 8 + uint16(o)] = res[i][j]
			}
		}

		s := ecuParams{vals: map[string]string{}}
		for _, p := range params {
			var v uint32
			var x uint32
			for _, a := range p.Addr {
				x = x << 8 + 0xff
				v = v << 8 + uint32(d[uint16(p.Page) << 8 + uint16(a)])
			}
			if p.Scale.ScaleMax > 0 {
				r := p.Scale.ScaleMax - p.Scale.ScaleMin
				f := float32(v) * r / float32(x) + p.Scale.ScaleMin
				s.vals[p.ID] = fmt.Sprintf("%s %s", strconv.FormatFloat(float64(f), 'f', p.Scale.Precision, 32), p.Scale.Units)
			} else if p.Bits != nil {
				flags := []int{}
				for b := range p.Bits {
					if b != 0 && v & uint32(b) == uint32(b) {
						flags = append(flags, int(b))
					}
				}
				sort.Ints(flags)
				f := make([]string, len(flags))
				for i, b := range(flags) {
					f[i] = fmt.Sprintf("%x: %s", b, p.Bits[uint16(b)])
				}
				if len(f) > 0 {
					s.vals[p.ID] = fmt.Sprintf("%s [%x]", strings.Join(f, ", "), v)
				} else {
					s.vals[p.ID] = fmt.Sprintf("0: %s [%x]", p.Bits[0], v)
				}
			} else {
				s.vals[p.ID] = fmt.Sprintf("%d", v)
			}
		}

		return s
	}
}

func pages(params []mbe.Param) []page {
	m := map[byte]map[byte]bool{}
	for _, p := range params {
		_, ok := m[p.Page]
		if !ok {
			m[p.Page] = map[byte]bool{}
		}
		for _, a := range p.Addr {
			m[p.Page][a] = true
		}
	}
	pgs := []page{}
	for pg, s := range m {
		p := page{pg: pg, addrs: []byte{}}
		for a := range s {
			p.addrs = append(p.addrs, a)
		}
		sort.Slice(p.addrs, func (i, j int) bool { return p.addrs[i] < p.addrs[j] })
		pgs = append(pgs, p)
	}
	sort.Slice(pgs, func (i, j int) bool { return pgs[i].pg < pgs[j].pg })
	return pgs
}
