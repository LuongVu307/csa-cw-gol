package gol

import (
	"strconv"
)

// Params provides the details of how to run the Game of Life and which image to load.
type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}

// Run starts the processing of Game of Life. It should initialise channels and goroutines.
func Run(p Params, events chan<- Event, keyPresses <-chan rune) {

	//	TODO: Put the missing channels in here.

	ioCommandChan := make(chan ioCommand)
	ioIdleChan := make(chan bool)
	ioFilenameChan := make(chan string)
	ioOutputChan := make(chan uint8)
	ioInputChan := make(chan uint8)

	ioChannels := ioChannels{
		command:  ioCommandChan,
		idle:     ioIdleChan,
		filename: ioFilenameChan,
		output:   ioOutputChan,
		input:    ioInputChan,
	}
	go startIo(p, ioChannels)

	distributorChannels := distributorChannels{
		events:     events,
		ioCommand:  ioCommandChan,
		ioIdle:     ioIdleChan,
		ioFilename: ioFilenameChan,
		ioOutput:   ioOutputChan,
		ioInput:    ioInputChan,
	}

	fileName := strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight)

	ioCommandChan <- ioInput
	ioFilenameChan <- fileName

	distributor(p, distributorChannels, keyPresses)
}
