package main

import (
	"log"
	"sync"
	"time"

	"github.com/stianeikeland/go-rpio"
)

type doorState string

const (
	doorOpen    doorState = "open"
	doorClosed  doorState = "closed"
	doorOpening doorState = "opening"
	doorClosing doorState = "closing"
	doorUnknown doorState = "unknown"
)

type gpio struct {
	doorOpen  rpio.Pin
	doorClose rpio.Pin
	doorMtx   sync.Mutex // can't open and close at the same time
	doorState doorState  // TODO
}

func newGPIO() *gpio {
	if err := rpio.Open(); err != nil {
		log.Fatal(err)
	}

	gpio := gpio{
		doorOpen:  20,
		doorClose: 21,
		doorMtx:   sync.Mutex{},
		doorState: doorUnknown,
	}
	gpio.doorOpen.Output()
	gpio.doorClose.Output()

	return &gpio
}

func (g *gpio) close() {
	if err := rpio.Close(); err != nil {
		log.Println(err)
	}
}

func (g *gpio) openDoor() {
	g.doorMtx.Lock()
	g.doorOpen.Low()
	g.doorClose.Low()

	g.doorState = doorOpening
	g.doorOpen.High()
	time.Sleep(22 * time.Second)
	g.doorOpen.Low()
	g.doorState = doorOpen

	g.doorMtx.Unlock()
}

func (g *gpio) closeDoor() {
	g.doorMtx.Lock()
	g.doorClose.Low()
	g.doorOpen.Low()

	g.doorState = doorClosing
	for i := 0; i < 9; i++ {
		g.doorClose.High()
		time.Sleep(800 * time.Millisecond)
		g.doorClose.Low()
		time.Sleep(2 * time.Second)
	}

	g.doorClose.High()
	time.Sleep(20 * time.Second)
	g.doorClose.Low()

	g.doorState = doorClosed
	g.doorMtx.Unlock()
}
