package gol

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
}

func modulo(b int, m int) int {
	v := b % m
	if v < 0 {
		return v + m
	} else {
		return v
	}
}

func calculateNextState(p Params, world [][]uint8) [][]uint8 {

	H := p.ImageHeight
	W := p.ImageWidth

	world2 := make([][]uint8, H)
	for i := range world2 {
		world2[i] = make([]uint8, W)
	}

	for h := 0; h < H; h++ {
		for w := 0; w < W; w++ {
			live := 0
			// vert + horiz
			if world[h][modulo(w+1, W)] == 255 {
				live++
			}
			if world[h][modulo(w-1, W)] == 255 {
				live++
			}
			if world[modulo(h+1, H)][w] == 255 {
				live++
			}
			if world[modulo(h-1, H)][w] == 255 {
				live++
			}

			//diag adj
			if world[modulo(h+1, H)][modulo(w+1, W)] == 255 {
				live++
			}
			if world[modulo(h+1, H)][modulo(w-1, W)] == 255 {
				live++
			}
			if world[modulo(h-1, H)][modulo(w+1, W)] == 255 {
				live++
			}
			if world[modulo(h-1, H)][modulo(w-1, W)] == 255 {
				live++
			}

			if world[h][w] == 255 && (live < 2 || live > 3) {
				world2[h][w] = 0
			} else if world[h][w] == 0 && live == 3 {
				world2[h][w] = 255
			} else {
				world2[h][w] = world[h][w]
			}
		}
	}
	return world2
}

func calculateNextStateForWorker(p Params, slice [][]uint8, resChan chan<- [][]uint8, idx int) {

	sliceHeight := len(slice)
	sliceWidth := len(slice[0])

	hBegin := 1
	hEnd := sliceHeight - 1

	slice2 := make([][]uint8, sliceHeight-2)
	for i := 0; i < sliceHeight-2; i++ {
		slice2[i] = make([]uint8, sliceWidth)
	}

	for h := hBegin; h < hEnd; h++ {
		for w := 0; w < sliceWidth; w++ {
			live := 0
			// vert + horiz
			if slice[h][modulo(w+1, sliceWidth)] == 255 {
				live++
			}
			if slice[h][modulo(w-1, sliceWidth)] == 255 {
				live++
			}
			if slice[h+1][w] == 255 {
				live++
			}
			if slice[h-1][w] == 255 {
				live++
			}

			//diag adj
			if slice[h+1][modulo(w+1, sliceWidth)] == 255 {
				live++
			}
			if slice[h+1][modulo(w-1, sliceWidth)] == 255 {
				live++
			}
			if slice[h-1][modulo(w+1, sliceWidth)] == 255 {
				live++
			}
			if slice[h-1][modulo(w-1, sliceWidth)] == 255 {
				live++
			}

			if slice[h][w] == 255 && (live < 2 || live > 3) {
				slice2[h-1][w] = 0
			} else if slice[h][w] == 0 && live == 3 {
				slice2[h-1][w] = 255
			} else {
				slice2[h-1][w] = slice[h][w]
			}
		}
	}
	resChan <- slice2
}

func calculateAliveCells(p Params, world [][]uint8) []util.Cell {
	alive := []util.Cell{}
	for h := 0; h < p.ImageHeight; h++ {
		for w := 0; w < p.ImageWidth; w++ {
			if world[h][w] == 255 {
				alive = append(alive, util.Cell{X: w, Y: h})
			}
		}
	}
	return alive
}
func calculateAliveCellsCount(p Params, world [][]uint8) int {
	alive := 0
	for h := 0; h < p.ImageHeight; h++ {
		for w := 0; w < p.ImageWidth; w++ {
			if world[h][w] == 255 {
				alive++
			}
		}
	}
	return alive
}

func startWorkers(p Params, world [][]uint8) [][]uint8 {
	threads := p.Threads
	height := p.ImageHeight
	width := p.ImageWidth

	if threads == 1 {
		return calculateNextState(p, world)
	}

	resultsArr := make([]chan [][]uint8, threads)
	for i := 0; i < threads; i++ {
		resultsArr[i] = make(chan [][]uint8)
	}
	{
		step := height / threads
		stepRemain := height % threads

		idx := 0
		first := 0
		last := first + step

		lastStep := step
		for idx < threads {
			if first == 0 {
				slice := make([][]uint8, last-first+2)
				slice[0] = world[height-1]
				for i := 0; i < last-first+1; i++ {
					slice[i+1] = world[i]
				}
				go calculateNextStateForWorker(p, slice, resultsArr[idx], idx)
			} else if last == height {
				slice := make([][]uint8, last-first+2)
				slice[last-first+1] = world[0]
				for i := 0; i < last-first+1; i++ {
					slice[i] = world[first-1+i]
				}
				go calculateNextStateForWorker(p, slice, resultsArr[idx], idx)
			} else {
				slice := world[first-1 : last+1]
				go calculateNextStateForWorker(p, slice, resultsArr[idx], idx)
			}

			nextStep := step
			if stepRemain != 0 {
				nextStep++
				stepRemain--
			}
			first += lastStep
			last += nextStep

			lastStep = nextStep

			idx++
			if last > height {
				last = height
			}
		}
		if first < height || idx != threads {
			fmt.Println("log: ", first, last, height, idx, threads)
			panic("internal error")
		}
	}

	res := make([][]uint8, height)
	for i := 0; i < height; i++ {
		res[i] = make([]uint8, width)
	}
	{
		idx := 0
		for i := 0; i < threads; i++ {
			slice := <-resultsArr[i]
			for j := 0; j < len(slice); j++ {
				res[idx] = slice[j]
				idx++
			}
		}
		if idx != height {
			fmt.Println("log: ", height, idx, threads)
			panic("internal error")
		}
	}
	return res
}

func outputWorker(p Params, c distributorChannels, world [][]uint8) {
	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			c.ioOutput <- world[i][j]
		}
	}
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels, keyPresses <-chan rune) {

	world := make([][]uint8, p.ImageHeight)
	for i := 0; i < p.ImageHeight; i++ {
		world[i] = make([]uint8, p.ImageWidth)
		for j := 0; j < p.ImageWidth; j++ {
			world[i][j] = <-c.ioInput
		}
	}

	c.events <- StateChange{0, Executing}

	// these are effectively binary semaphores
	doneSig := make(chan bool)
	incSig := make(chan bool)

	curTurn := 0
	var worldMtx sync.Mutex

	endFg := int32(0)
	pauseFg := int32(0)
	go func() {
		for i := 0; i < p.Turns && atomic.LoadInt32(&endFg) == 0; i++ {
			for atomic.LoadInt32(&pauseFg)
			worldMtx.Lock()
			world = startWorkers(p, world)
			worldMtx.Unlock()
			incSig <- true
		}
		doneSig <- true
	}()

	timer := time.NewTicker(2 * time.Second)

	contSelect := true
	for contSelect {
		select {
		case <-incSig:
			{
				curTurn++
			}
		case <-doneSig:
			{
				contSelect = false

			}
		case <-timer.C:
			{
				worldMtx.Lock()
				c.events <- AliveCellsCount{curTurn, calculateAliveCellsCount(p, world)}
				worldMtx.Unlock()

			}
		case key := <-keyPresses:
			{
				switch key {
				case 's':
					{
						worldMtx.Lock()
						outputWorker(p, c, world)
						worldMtx.Unlock()

						// c.ioCommand <- ioCheckIdle
						// <-c.ioIdle

						c.events <- ImageOutputComplete{curTurn, "dummyfile"}
					}
				case 'q':
					{
						contSelect = false
						atomic.StoreInt32(&endFg, int32(0))
					}
				case 'p':
					{
						// TODO pause
					}
				}
			}
		}
	}

	c.events <- FinalTurnComplete{p.Turns, calculateAliveCells(p, world)}
	c.ioCommand <- ioOutput
	// no point in using "go" keyword, the current goroutine will already block until IO is done
	outputWorker(p, c, world)

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	if p.Threads == 1 && p.ImageHeight <= 64 {
		fmt.Println("DISTRIBUTOR OUTPUT: \n", util.AliveCellsToString(calculateAliveCells(p, world), calculateAliveCells(p, world), p.ImageWidth, p.ImageHeight))
	}

	c.events <- StateChange{p.Turns, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
