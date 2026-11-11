package gol

import (
	"fmt"
	"net/rpc"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"uk.ac.bris.cs/gameoflife/util"
)

// Params provides the details of how to run the Game of Life and which image to load.
type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}

// FOR PARALLEL HEAP ALLOCATION
func heapAllocationDefaultWorker(slice [][]uint8, resChan chan<- [][]uint8) {
	resChan <- calculateResultSliceForWorkerHeapAllocation(slice)
}

func distributeWithHeapAllocation(p Params, c distributorChannels, keyPresses <-chan rune) {
	heapAllocationTurnFn := func(p Params, world *[][]uint8, completedTurns int) {
		world2 := startWorkersHeapAllocation(p, *world, heapAllocationDefaultWorker)
		for y := 0; y < p.ImageHeight; y++ {
			for x := 0; x < p.ImageWidth; x++ {
				if (*world)[y][x] != world2[y][x] {
					c.events <- CellFlipped{
						CompletedTurns: completedTurns + 1,
						Cell: util.Cell{X: x, Y: y},
					}
				}
			}
		}
		*world = world2
	}
	distributor(p, c, keyPresses, heapAllocationTurnFn)
}

// FOR PARALLEL DOUBLE BUFFER
func defaultWorker(slice [][]uint8, destination [][]uint8, done chan<- struct{}) {
	calculateResultSliceForWorker(slice, destination)
	done <- struct{}{}
}
func defaultWorkerSingleThread(p Params, world [][]uint8, res [][]uint8) {
	calculateNextState(p, world, res)
}
func distributeWithDoubleBuffer(p Params, c distributorChannels, keyPresses <-chan rune) {

	world2 := make2DSlice(p.ImageWidth, p.ImageHeight)
	doubleBufferTurnFn := func(p Params, world *[][]uint8, completedTurns int) {
		startWorkersDoubleBuffer(p, *world, world2, defaultWorkerSingleThread, defaultWorker)

		// Send the CellFlipped after every turn completed
		for y := 0; y < p.ImageHeight; y++ {
			for x := 0; x < p.ImageWidth; x++ {
				if (*world)[y][x] != world2[y][x] {
					c.events <- CellFlipped{
						CompletedTurns: completedTurns,
						Cell: util.Cell{X: x, Y: y},
					}
				}
			}
		}

		*world, world2 = world2, *world
	}

	distributor(p, c, keyPresses, doubleBufferTurnFn)
}

// FOR PARALLEL MAP REDUCE
func mapReduceDefaultWorker(startY int, slice [][]uint8, resChan chan util.Cell) {
	cs := calculateFlippedCellsForWorkerMapReduce(slice, startY)
	for i := 0; i < len(cs); i++ {
		resChan <- cs[i]
	}
}
func mapReduceDefaultWorkerSingleThread(p Params, world [][]uint8) []util.Cell {
	return calculateCellsFlippedMapReduce(p, world)
}
func distributeWithMapReduce(p Params, c distributorChannels, keyPresses <-chan rune) {
	cellsFlippedBuffer := make([]util.Cell, 0)
	mapReduceTurnFn := func(p Params, world *[][]uint8, completedTurns int) {
		size := 0

		startWorkersMapReduce(p, *world, mapReduceDefaultWorkerSingleThread, mapReduceDefaultWorker, &cellsFlippedBuffer, &size)

		for _, cell := range cellsFlippedBuffer[:size] {
			c.events <- CellFlipped{
				CompletedTurns: completedTurns + 1,
				Cell: cell,
			}
		}

	}

	distributor(p, c, keyPresses, mapReduceTurnFn)
}

// FOR DISTRIBUTED MAP REDUCE

type BrokerArgs struct {
	P     Params
	World [][]uint8
}

type CompleteResult struct {
	Turn int
	Res  [][]uint8
}
type SaveResult struct {
	Turn int
	Res  [][]uint8
}
type QuitResult struct {
	Turn int
	Res  [][]uint8
}
type PauseUnpauseResult struct {
	Turns   int
	IsPause bool
}
type CellCountResult struct {
	Turn  int
	Count int
}

func distributeWithBroker(p Params, c distributorChannels, keyPresses <-chan rune) {
	bytes, err := os.ReadFile("brokerURL.txt")
	if err != nil {
		panic("brokerURL.txt not found")
	}
	// theres some newline character in the filename, this just removes it
	url := strings.ReplaceAll(string(bytes), "\n", "")
	fmt.Println("Connecting to server at (", url, ")")
	client, err := rpc.Dial("tcp", url)
	fmt.Println("Attempted connection")
	if err != nil {
		panic("Failed to connect to broker")
	} else {
		fmt.Println("Connection successful")
	}

	world := make2DSlice(p.ImageWidth, p.ImageHeight)
	populateWorld(world, p.ImageWidth, p.ImageHeight, c)

	doneSig := make(chan bool, 1)
	ticker := time.NewTicker(2 * time.Second)

	var wg sync.WaitGroup
	wg.Go(func() {
		contLoop := true
		for contLoop {
			select {
			case <-ticker.C:
				var res CellCountResult
				client.Call("Broker.ProcessCellCount", 0, &res)
				c.events <- AliveCellsCount{
					CompletedTurns: res.Turn,
					CellsCount:     res.Count,
				}
			case key := <-keyPresses:
				switch key {
				case 's':
					var res SaveResult
					client.Call("Broker.ProcessSave", 0, &res)
					fmt.Println("Alive in save result: ", calculateAliveCellsCount(p, res.Res))
					filename := distributorQuitFilenameFn(p.ImageWidth, p.ImageHeight, res.Turn)

					c.ioCommand <- ioOutput
					c.ioFilename <- filename
					outputWorker(p, c, res.Res)

					c.ioCommand <- ioCheckIdle
					<-c.ioIdle

					c.events <- ImageOutputComplete{res.Turn, filename}
				case 'q':
					var res QuitResult
					client.Call("Broker.ProcessQuit", 0, &res)
					distributorExitFn(p, c, res.Res, res.Turn, distributorQuitFilenameFn(p.ImageWidth, p.ImageHeight, res.Turn))
					<-doneSig
					contLoop = false
				case 'p':
					var res PauseUnpauseResult
					client.Call("Broker.ProcessPauseUnpause", 0, &res)
					if res.IsPause {
						c.events <- StateChange{res.Turns, Paused}
					} else {
						c.events <- StateChange{res.Turns, Executing}
					}
				}

			case <-doneSig:
				contLoop = false
			}
		}
	})

	c.events <- StateChange{0, Executing}
	args := BrokerArgs{P: p, World: world}
	var res CompleteResult
	client.Call("Broker.ProcessConnection", args, &res)
	doneSig <- true

	wg.Wait()
	if res.Res != nil {
		distributorExitFn(p, c, res.Res, res.Turn, distributorDefaultExitFilenameFn(p))
	}
	close(c.events)
}

func Run(p Params, events chan<- Event, keyPresses <-chan rune) {
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
	// distributeWithHeapAllocation(p, distributorChannels, keyPresses)
	// distributeWithDoubleBuffer(p, distributorChannels, keyPresses)
	distributeWithMapReduce(p, distributorChannels, keyPresses)
	// distributeWithBroker(p, distributorChannels, keyPresses)
}
