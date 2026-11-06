package gol

import (
	"strconv"
	"uk.ac.bris.cs/gameoflife/util"
)

// Params provides the details of how to run the Game of Life and which image to load.
type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}

func distributeWithDoubleBuffer(p Params, c distributorChannels, keyPresses <-chan rune) {

	world2 := make2DSlice(p.ImageWidth, p.ImageHeight)
	doubleBufferTurnFn := func(p Params, world *[][]uint8) {
		startWorkersDoubleBuffer(p, *world, world2)

		*world, world2 = world2, *world
	}

	distributor(p, c, keyPresses, doubleBufferTurnFn)
}
func distributeWithMapReduce(p Params, c distributorChannels, keyPresses <-chan rune) {
	cellsFlippedBuffer := make([]util.Cell, 0)
	mapReduceTurnFn := func(p Params, world *[][]uint8) {
		size := 0
		startWorkersMapReduce(p, *world, &cellsFlippedBuffer, &size)
	}

	distributor(p, c, keyPresses, mapReduceTurnFn)
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

	// TODO uncomment the preferred function below (and comment out the rest)
	distributeWithDoubleBuffer(p, distributorChannels, keyPresses)
	// distributeWithMapReduce(p, distributorChannels, keyPresses)
}
