package main

import (
	"encoding/hex"
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

var timeout = time.Millisecond * 250
var maxStaleness = time.Second * 3

var title = lipgloss.NewStyle().
	Bold(true).
	BorderStyle(lipgloss.DoubleBorder()).
	Padding(1).
	Width(50).
	AlignHorizontal(lipgloss.Center)

var errText = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9"))

type model struct {
	con mbe.Conn

	params []mbe.Param
	pages []page

	ver version

	state ecuParams

	err error
	raw bool
}

type version struct {
	ver string
}

type ecuParams struct {
	ts   time.Time
	vals map[string]paramVal
}

type paramVal struct{
	val string
	raw string
}

type commErr struct {
	err error
}

type ready struct {
	con mbe.Conn
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
		case "r":
			m.raw = !m.raw
			return m, nil
		}
	case version:
		m.err = nil
		m.ver = msg
		return m, poll(m.con, m.pages, m.params)
	case ecuParams:
		m.err = nil
		m.state = msg
		return m, poll(m.con, m.pages, m.params)
	case commErr:
		m.err = msg.err
		return m, reset(m.con)
	case ready:
		m.con = msg.con
		return m, identify(m.con)
	}
	return m, nil
}

func (m model) View() string {
	s := title.Render("Caterscan") + "\n"
	if m.err != nil {
		s += errText.Render(fmt.Sprintf("%25s  %v", "Error", m.err)) + "\n"
	}
	if m.ver.ver != "" {
		s += fmt.Sprintf("%25s  %s\n", "Serial", m.ver.ver)
	}
	vals := m.state.vals
	if time.Now().After(m.state.ts.Add(maxStaleness)) {
		vals = map[string]paramVal{}
	}
	for _, param := range m.params {
		val, ok := vals[param.ID]
		if ok {
			if m.raw {
				s += fmt.Sprintf("%25s  %-8s  %s\n", param.Name, val.raw, val.val)
			} else {
				s += fmt.Sprintf("%25s  %s\n", param.Name, val.val)
			}
		} else {
			s += fmt.Sprintf("%25s  nil\n", param.Name)
		}
	}
	return s
}

func identify(con mbe.Conn) tea.Cmd {
	return func() tea.Msg {
		con.SetTimeout(timeout)
		if err := con.Send(mbe.VersionReq); err != nil {
			return commErr{err: err}
		}
		res, err := con.Recv()
		if err != nil {
			return commErr{err: err}
		}
		ver, err := mbe.ParseVersionResponse(res)
		if err != nil {
			return commErr{err: err}
		}
		return version{ver: ver}
	}
}

func poll(con mbe.Conn, pages []page, params []mbe.Param) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(50 * time.Millisecond)
		con.SetTimeout(timeout)

		reqs := make([][]byte, len(pages))
		for n, p := range pages {
			reqs[n] = mbe.CreateDataRequest(p.pg, p.addrs)
		}

		res := make([][]byte, len(reqs))
		for i, req := range reqs {
			if err := con.Send(req); err != nil {
				return commErr{err: err}
			}
			r, err := con.Recv()
			if err != nil {
				return commErr{err: err}
			}
			res[i], err = mbe.ParseDataResponse(r)
			if err != nil {
				return commErr{err: err}
			}
		}

		d := map[uint16]byte{}
		for i, p := range pages {
			for j, o := range p.addrs {
				d[uint16(p.pg) << 8 + uint16(o)] = res[i][j]
			}
		}

		s := ecuParams{ts: time.Now(), vals: map[string]paramVal{}}
		for _, p := range params {
			var v uint32
			var x uint32
			h := []byte{}
			for _, a := range p.Addr {
				b := d[uint16(p.Page) << 8 + uint16(a)]
				x = x << 8 + 0xff
				v = v << 8 + uint32(b)
				h = append(h, b)
			}
			var val string
			raw := hex.EncodeToString(h)
			if p.Scale.ScaleMax > 0 {
				r := p.Scale.ScaleMax - p.Scale.ScaleMin
				f := float32(v) * r / float32(x) + p.Scale.ScaleMin
				val = fmt.Sprintf("%s %s", strconv.FormatFloat(float64(f), 'f', p.Scale.Precision, 32), p.Scale.Units)
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
					val = fmt.Sprintf("%s [%x]", strings.Join(f, ", "), v)
				} else {
					val = fmt.Sprintf("0: %s [%x]", p.Bits[0], v)
				}
			} else {
				val = fmt.Sprintf("%d", v)
			}
			s.vals[p.ID] = paramVal{val: val, raw: raw}
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

func reset(con mbe.Conn) tea.Cmd {
	return func() tea.Msg {
		err, nc := con.Reopen()
		if err != nil {
			return commErr{err: err}
		}
		return ready{con: nc}
	}
}
