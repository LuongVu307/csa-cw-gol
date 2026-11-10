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

func calculateAdjacentLiveCellCount(w int, h int, W int, H int, world [][]uint8) int {
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

	return live
}

// this differs to the above function as the input slice has an extra row at the top and bottom of the actual slice to be modified
func calculateAdjacentLiveCellCountForWorker(w int, h int, sliceWidth int, slice [][]uint8) int {
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
	return live
}

func calculateNextState(p Params, world [][]uint8, res [][]uint8) {
	H := p.ImageHeight
	W := p.ImageWidth

	for h := 0; h < H; h++ {
		for w := 0; w < W; w++ {
			live := calculateAdjacentLiveCellCount(w, h, W, H, world)
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
func calculateCellsFlippedMapReduce(p Params, world [][]uint8) []util.Cell {
	H := p.ImageHeight
	W := p.ImageWidth

	// just a starting point ig
	cellsFlipped := make([]util.Cell, 0)

	for h := 0; h < H; h++ {
		for w := 0; w < W; w++ {
			live := calculateAdjacentLiveCellCount(w, h, W, H, world)
			if world[h][w] == 255 && (live < 2 || live > 3) {
				cellsFlipped = append(cellsFlipped, util.Cell{X: w, Y: h})
			} else if world[h][w] == 0 && live == 3 {
				cellsFlipped = append(cellsFlipped, util.Cell{X: w, Y: h})
			}
		}
	}

	return cellsFlipped
}

func calculateResultSliceForWorker(slice [][]uint8) [][]uint8 {

	sliceHeight := len(slice)

	// should be equal to the width of the overall grid, but i dont wanna change the variable :(
	// TODO maybe use the ImageWidth from Params for sliceWidth
	sliceWidth := len(slice[0])

	hBegin := 1
	hEnd := sliceHeight - 1

	slice2 := make([][]uint8, sliceHeight-2)
	for i := 0; i < sliceHeight-2; i++ {
		slice2[i] = make([]uint8, sliceWidth)
	}

	for h := hBegin; h < hEnd; h++ {
		for w := 0; w < sliceWidth; w++ {
			live := calculateAdjacentLiveCellCountForWorker(w, h, sliceWidth, slice)
			if slice[h][w] == 255 && (live < 2 || live > 3) {
				slice2[h-1][w] = 0
			} else if slice[h][w] == 0 && live == 3 {
				slice2[h-1][w] = 255
			} else {
				slice2[h-1][w] = slice[h][w]
			}
		}
	}
	return slice2
}
func calculateFlippedCellsForWorkerMapReduce(slice [][]uint8, startY int) []util.Cell {
	sliceHeight := len(slice)

	// should be equal to the width of the overall grid, but i dont wanna change the variable :(
	// TODO maybe use the ImageWidth from Params for sliceWidth
	sliceWidth := len(slice[0])

	hBegin := 1
	hEnd := sliceHeight - 1

	cellsFlipped := make([]util.Cell, 0)

	for h := hBegin; h < hEnd; h++ {
		for w := 0; w < sliceWidth; w++ {
			live := calculateAdjacentLiveCellCountForWorker(w, h, sliceWidth, slice)
			if slice[h][w] == 255 && (live < 2 || live > 3) {
				cellsFlipped = append(cellsFlipped, util.Cell{X: w, Y: startY + h - 1})
			} else if slice[h][w] == 0 && live == 3 {
				cellsFlipped = append(cellsFlipped, util.Cell{X: w, Y: startY + h - 1})
			}
		}
	}
	return cellsFlipped
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

// the "worker" parameter takes in a slice with an extra row on the top and bottom,
// which returns a result slice WITHOUT the extra rows.
// the "worker1" parameter takes in "world" and writes to the "res" slice
//
// "worker1" is the worker that is used when p.Threads == 1
// "worker" is the worker that is used when p.Threads > 1
func startWorkersDoubleBuffer(p Params, world [][]uint8, res [][]uint8,
	worker1 func(p Params, world [][]uint8, res [][]uint8),
	worker func(slice [][]uint8, resChan chan [][]uint8)) {
	threads := p.Threads
	height := p.ImageHeight
	// width := p.ImageWidth

	if threads == 1 {
		// calculateNextState(p, world, res)

		worker1(p, world, res)

		return
	}
	resultsArr := make([]chan [][]uint8, threads)
	for i := 0; i < threads; i++ {
		resultsArr[i] = make(chan [][]uint8, 1)
	}
	{
		// worker := func(slice [][]uint8, resChan chan [][]uint8) {
		// 	resChan <- calculateResultSliceForWorker(slice)
		// }
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
				go worker(slice, resultsArr[idx])
			} else if last == height {
				slice := make([][]uint8, last-first+2)
				slice[last-first+1] = world[0]
				for i := 0; i < last-first+1; i++ {
					slice[i] = world[first-1+i]
				}
				go worker(slice, resultsArr[idx])
			} else {
				slice := world[first-1 : last+1]
				go worker(slice, resultsArr[idx])
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

func sizedAppend(slice *[]util.Cell, sizeP *int, val util.Cell) {
	if *sizeP == len(*slice) {
		*slice = append(*slice, val)
	} else {
		(*slice)[*sizeP] = val
	}
	(*sizeP)++
}

// "worker1" is the worker that is used when p.Threads == 1
// "worker" is the worker that is used when p.Threads > 1
func startWorkersMapReduce(p Params, world [][]uint8,
	worker1 func(p Params, slice [][]uint8) []util.Cell,
	worker func(startY int, slice [][]uint8, resChan chan util.Cell),
	cellsFlipped *[]util.Cell, sizeP *int) {
	threads := p.Threads
	height := p.ImageHeight
	// width := p.ImageWidth

	channelBufferSize := 10

	if threads == 1 {
		// calculateNextStateMapReduce(p, world)

		cs := worker1(p, world)
		for i := 0; i < len(cs); i++ {
			cell := cs[i]
			world[cell.Y][cell.X] = 255 - world[cell.Y][cell.X]
		}
	} else {

		cellFlippedChan := make(chan util.Cell, channelBufferSize)
		var wg sync.WaitGroup
		{
			step := height / threads
			stepRemain := height % threads

			idx := 0
			first := 0
			last := first + step

			lastStep := step

			makeWorker := func(startY int, slice [][]uint8, resChan chan util.Cell) func() {
				return func() {
					worker(startY, slice, resChan)
				}
				// return func() {
				// 	cs := calculateFlippedCellsForWorkerMapReduce(slice, startY)
				// 	for i := 0; i < len(cs); i++ {
				// 		resChan <- cs[i]
				// 	}
				// }
			}

			for idx < threads {
				if first == 0 {
					slice := make([][]uint8, last-first+2)
					slice[0] = world[height-1]
					for i := 0; i < last-first+1; i++ {
						slice[i+1] = world[i]
					}
					wg.Go(makeWorker(first, slice, cellFlippedChan))
				} else if last == height {
					slice := make([][]uint8, last-first+2)
					slice[last-first+1] = world[0]
					for i := 0; i < last-first+1; i++ {
						slice[i] = world[first-1+i]
					}
					wg.Go(makeWorker(first, slice, cellFlippedChan))
				} else {
					slice := world[first-1 : last+1]
					wg.Go(makeWorker(first, slice, cellFlippedChan))
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

		doneSig := make(chan bool)
		go func() {
			wg.Wait()
			doneSig <- true
			close(cellFlippedChan)
		}()

		{
			contLoop := true
			for contLoop {
				select {
				case cell := <-cellFlippedChan:
					sizedAppend(cellsFlipped, sizeP, cell)
				case <-doneSig:
					contLoop = false
				}
			}
			for cell := range cellFlippedChan {
				sizedAppend(cellsFlipped, sizeP, cell)
			}
			for i := 0; i < *sizeP; i++ {
				cell := (*cellsFlipped)[i]
				world[cell.Y][cell.X] = 255 - world[cell.Y][cell.X]
			}
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
		res[i] = make([]uint8, x)
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

func distributorQuitFilenameFn(x int, y int, turnsDone int) string {
	return strconv.Itoa(x) + "x" + strconv.Itoa(y) + "x" + strconv.Itoa(turnsDone)
}

func distributorExitFn(p Params, c distributorChannels, world [][]uint8, turnsDone int, filename string) {
	c.events <- FinalTurnComplete{turnsDone, calculateAliveCells(p, world)}
	c.ioCommand <- ioOutput
	c.ioFilename <- filename
	// no point in using "go" keyword, the current goroutine will anyways block until IO is done
	outputWorker(p, c, world)

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- ImageOutputComplete{turnsDone, filename}

	c.events <- StateChange{turnsDone, Quitting}

}

func distributorDefaultExitFilenameFn(p Params) string {
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
func distributor(p Params, c distributorChannels, keyPresses <-chan rune, completeTurn func(p Params, world *[][]uint8, completedTurns int)) {

	world := make2DSlice(p.ImageWidth, p.ImageHeight)
	populateWorld(world, p.ImageWidth, p.ImageHeight, c)

	// Sending the CellFLippe once when the world is populated
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			if world[y][x] == 255 {
				c.events <- CellFlipped{
					CompletedTurns: 0,
					Cell:           util.Cell{X: x, Y: y},
				}
			}
		}
	}

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
			case <-finishCmdSig:
				distributorExitFn(p, c, world, i, distributorQuitFilenameFn(p.ImageWidth, p.ImageHeight, i))
				contLoop = false
			case _ = <-saveCmdSig:
				filename := distributorQuitFilenameFn(p.ImageWidth, p.ImageHeight, i)

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
					completeTurn(p, &world, i)
					i++
					c.events <- TurnComplete{CompletedTurns: i}
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
		distributorExitFn(p, c, world, p.Turns, distributorDefaultExitFilenameFn(p))
	}
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
