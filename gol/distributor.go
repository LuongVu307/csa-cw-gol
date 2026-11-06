package gol

import (
	"fmt"
	"strconv"
	"sync"
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

func calculateNextState(p Params, world [][]uint8, res [][]uint8) {

	H := p.ImageHeight
	W := p.ImageWidth

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
				res[h][w] = 0
			} else if world[h][w] == 0 && live == 3 {
				res[h][w] = 255
			} else {
				res[h][w] = world[h][w]
			}
		}
	}
}

func calculateNextStateForWorker(slice [][]uint8, resChan chan<- [][]uint8) {

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

func startWorkers(p Params, world [][]uint8, res [][]uint8) {
	threads := p.Threads
	height := p.ImageHeight
	// width := p.ImageWidth

	if threads == 1 {
		calculateNextState(p, world, res)
		return
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
				go calculateNextStateForWorker(slice, resultsArr[idx])
			} else if last == height {
				slice := make([][]uint8, last-first+2)
				slice[last-first+1] = world[0]
				for i := 0; i < last-first+1; i++ {
					slice[i] = world[first-1+i]
				}
				go calculateNextStateForWorker(slice, resultsArr[idx])
			} else {
				slice := world[first-1 : last+1]
				go calculateNextStateForWorker(slice, resultsArr[idx])
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
}

func outputWorker(p Params, c distributorChannels, world [][]uint8) {
	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			c.ioOutput <- world[i][j]
		}
	}
}

func make2DSlice(x int, y int) [][]uint8 {
	res := make([][]uint8, y)
	for i := 0; i < y; i++ {
		res[y] = make([]uint8, x)
	}
	return res
}

func populateWorld(world [][]uint8, x int, y int, c distributorChannels) {
	for i := 0; i < y; i++ {
		for j := 0; j < x; j++ {
			world[i][j] = <-c.ioInput
		}
	}
}

func quitFilenameFn(x int, y int, turnsDone int) string {
	return strconv.Itoa(x) + "x" + strconv.Itoa(y) + "x" + strconv.Itoa(turnsDone)
}

func exitFn(p Params, c distributorChannels, world [][]uint8, turnsDone int, filename string) {
	c.events <- FinalTurnComplete{p.Turns, calculateAliveCells(p, world)}
	c.ioCommand <- ioOutput
	c.ioFilename <- filename
	// no point in using "go" keyword, the current goroutine will anyways block until IO is done
	outputWorker(p, c, world)

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- ImageOutputComplete{turnsDone, filename}

	c.events <- StateChange{p.Turns, Quitting}
}

func defaultExitFilenameFn(p Params) string {
	defaultExitFilename := strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight)
	if p.Turns == 100 {
		defaultExitFilename += "x100"
	} else if p.Turns == 0 {
		defaultExitFilename += "x0"
	} else {
		defaultExitFilename += "x" + strconv.Itoa(p.Threads)
	}
	return defaultExitFilename
}

// distributor divides the work between workers and interacts with other goroutines.
func distributorDoubleBuffer(p Params, c distributorChannels, keyPresses <-chan rune) {

	world := make2DSlice(p.ImageWidth, p.ImageHeight)
	populateWorld(world, p.ImageWidth, p.ImageHeight, c)
	world2 := make2DSlice(p.ImageWidth, p.ImageHeight)

	doneSig := make(chan bool)
	pauseUnpauseCmdSig := make(chan bool)
	finishCmdSig := make(chan bool)
	saveCmdSig := make(chan bool)

	loopFn := func() {
		ticker := time.NewTicker(2 * time.Second)
		i := 0
		contLoop := true
		paused := false

		for {
			if i == p.Turns {
				doneSig <- true
				break
			} else if !contLoop {
				break
			}

			select {
			case _ = <-pauseUnpauseCmdSig:
				if paused {
					// unpause
					paused = false
					c.events <- StateChange{i, Executing}
				} else {
					// pause
					paused = true
					c.events <- StateChange{i, Paused}
				}
			case _ = <-finishCmdSig:
				exitFn(p, c, world, i, quitFilenameFn(p.ImageWidth, p.ImageHeight, i))
				contLoop = false
			case _ = <-saveCmdSig:
				filename := quitFilenameFn(p.ImageWidth, p.ImageHeight, i)

				c.ioCommand <- ioOutput
				c.ioFilename <- filename
				outputWorker(p, c, world)

				c.ioCommand <- ioCheckIdle
				<-c.ioIdle

				c.events <- ImageOutputComplete{i, filename}
			case _ = <-ticker.C:
				c.events <- AliveCellsCount{i, calculateAliveCellsCount(p, world)}
			default:
				if !paused {
					startWorkers(p, world, world2)

					// TODO check if this makes a copy
					// if it does make a copy, change it
					tmp := world2
					world2 = world
					world = tmp

					i++
				}
			}

		}
	}
	var wg sync.WaitGroup
	wg.Go(loopFn)

	c.events <- StateChange{0, Executing}

	contSelect := true
	quitFlag := false
	for contSelect {
		select {
		case <-doneSig:
			{
				contSelect = false
			}
		case key := <-keyPresses:
			{
				switch key {
				case 's':
					saveCmdSig <- true
				case 'q':
					finishCmdSig <- true
					// other thread is responsible for calling exitFn
					quitFlag = true
					contSelect = false
				case 'p':
					pauseUnpauseCmdSig <- true
				}
			}
		}
	}
	wg.Wait()

	if !quitFlag {
		exitFn(p, c, world, p.Turns, defaultExitFilenameFn(p))
	}
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
