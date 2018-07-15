// Package jukebox provides a set of tools for interacting with mpd
package jukebox

import (
	"bufio"
	"fmt"
	"net"
	"strings"
)

type Player interface {
	Add(uri string) error
}

type MPD struct {
	conn    net.Conn
	s       *bufio.Scanner
	Version string
	resp    chan string
	cmds    chan command
}

func NewMPD(c net.Conn) *MPD {
	m := &MPD{
		conn: c,
		resp: make(chan string),
		cmds: make(chan command),
	}
	go m.scan()
	start := startState(m)
	go m.run(start)
	return m
}

func (m *MPD) Add(uri string) error {
	c := newCommand("add", uri)
	m.cmds <- c
	return <-c.err
}

func (m *MPD) scan() {
	m.s = bufio.NewScanner(m.conn)
	for more := m.s.Scan(); more; more = m.s.Scan() {
		m.resp <- m.s.Text()
	}
}

func (m *MPD) run(start stateFn) {
	if start == nil {
		start = startState
	}
	for state := start; state != nil; {
		state = state(m)
	}
}

type command struct {
	Type string
	Params string
	err chan error
}

func newCommand(t, p string) command {
	return command{
		Type: t,
		Params: p,
		err: make(chan error),
	}
}

type stateFn func(m *MPD) stateFn

var startState = connecting

func connecting(m *MPD) stateFn {
	resp := <-m.resp
	n, err := fmt.Sscanf(resp, "OK MPD %s", &m.Version)
	if err != nil || n != 1 {
		return unrecoverable("unexpected response", nil)
	}
	return connected
}

func connected(m *MPD) stateFn {
	select {
	case cmd := <-m.cmds:
		return commandHandler(cmd)
	case <-m.resp:
		return unrecoverable("unexpected response", nil)
	}
}

func unrecoverable(msg string, errChan chan<- error) stateFn {
	return func(*MPD) stateFn {
		if errChan != nil {
			errChan <- fmt.Errorf("unrecoverable error: %s", msg)
		}
		fmt.Println(msg)
		return nil
	}
}

func recoverable(msg string, errChan chan<- error) stateFn {
	return func(*MPD) stateFn {
		if errChan != nil {
			errChan <- fmt.Errorf("recoverable error: %s", msg)
		}
		fmt.Println(msg)
		return connected
	}
}

func commandHandler(cmd command) stateFn {
	switch cmd.Type {
	case "add":
		return add(cmd.Params, cmd.err)
	case "idle":
		return idling(cmd.Params, cmd.err)
	default:
		return connected
	}
}

func add(params string, errChan chan<- error) stateFn {
	return func(m *MPD) stateFn {
		fmt.Fprintf(m.conn, "add %s\n", params)
		r := <-m.resp
		if strings.HasPrefix("ACK", r) {
			return recoverable(r, errChan)
		}
		if !strings.HasPrefix("OK", r) {
			return unrecoverable("unexpected response", errChan)
		}
		close(errChan)
		return connected
	}
}

func idling(params string, errChan chan<- error) stateFn {
	return func(m *MPD) stateFn {
		fmt.Fprintf(m.conn, "idle %s\n", params)
		select {
		case cmd := <-m.cmds:
			if cmd.Type != "noidle" {
				panic("don't do that!")
			}
			fmt.Fprintf(m.conn, "noidle\n")
			close(errChan)
			return connected
		case resp := <-m.resp:
			var system []string
			for resp != "OK" {
				var sys string
				n, err := fmt.Sscanf(resp, "changed: %s", &sys)
				if n != 1 || err != nil {
					return unrecoverable("bad response", errChan)
				}
				system = append(system, sys)
				resp = <-m.resp
			}
			close(errChan)
			return connected
		}
	}
}
