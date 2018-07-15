// Package main provides an autoqueue util
package main

import (
	"fmt"
	"net"

	"github.com/kitschysynq/wbtc-jukebox"
)

func main() {
	c, err := net.Dial("tcp", ":6600")
	if err != nil {
		panic(err)
	}

	m := jukebox.NewMPD(c)
	fmt.Printf("connected to mpd version %q\n", m.Version)
	err = m.Add("\"The Mothers of Invention\"")
	if err != nil {
		panic(err)
	}
}
