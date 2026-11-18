package gol

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"uk.ac.bris.cs/gameoflife/util"
)

type benchmarkStrategy struct {
	name string
	run  func(Params, distributorChannels, <-chan rune)
}

var parallelBenchmarkStrategies = []benchmarkStrategy{
	{"heap", distributeWithHeapAllocation},
	{"double", distributeWithDoubleBuffer},
	{"mapreduce", distributeWithMapReduce},
}
var parallelBenchmarkWorkers = []int{1, 2, 4, 8, 12, 16}

// benchmarkImage reads the supplied binary PGM fixtures before timing starts.
func benchmarkImage(tb testing.TB, path string, width, height int) []byte {
	tb.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		tb.Fatal(err)
	}
	parts := bytes.SplitN(data, []byte{'\n'}, 4)
	if len(parts) != 4 || string(parts[0]) != "P5" || string(parts[2]) != "255" {
		tb.Fatalf("unsupported PGM header: %s", path)
	}
	var w, h int
	if _, err := fmt.Sscanf(string(parts[1]), "%d %d", &w, &h); err != nil || w != width || h != height || len(parts[3]) != width*height {
		tb.Fatalf("invalid PGM dimensions/data: %s", path)
	}
	return parts[3]
}

type benchmarkResult struct {
	alive     []util.Cell
	finalTurn int
	turns     int
}

// runParallelBenchmark uses the actual local distributors, with finite in-memory
// I/O and an event consumer instead of disk operations and SDL. Each invocation
// creates a fresh board from the same pixels and waits for both goroutines.
func runParallelBenchmark(p Params, pixels []byte, strategy benchmarkStrategy) benchmarkResult {
	events := make(chan Event, 1000) // Same capacity as main.go.
	commands := make(chan ioCommand)
	filenames := make(chan string)
	input := make(chan uint8)
	output := make(chan uint8)
	idle := make(chan bool)
	ioDone := make(chan struct{})
	go func() {
		defer close(ioDone)
		for _, pixel := range pixels {
			input <- pixel
		}
		// With no keyboard input the distributor performs exactly one final output.
		<-commands // ioOutput
		<-filenames
		for range pixels {
			<-output
		}
		<-commands // ioCheckIdle
		idle <- true
	}()
	channels := distributorChannels{
		events: events, ioCommand: commands, ioIdle: idle,
		ioFilename: filenames, ioOutput: output, ioInput: input,
	}
	distributorDone := make(chan struct{})
	go func() { strategy.run(p, channels, nil); close(distributorDone) }()
	var result benchmarkResult
	for event := range events {
		switch event := event.(type) {
		case FinalTurnComplete:
			result.alive, result.finalTurn = event.Alive, event.CompletedTurns
		case TurnComplete:
			result.turns++
		}
	}
	<-distributorDone
	<-ioDone
	return result
}

func BenchmarkParallel(b *testing.B) {
	for _, size := range []int{128, 512} {
		pixels := benchmarkImage(b, filepath.Join("..", "images", fmt.Sprintf("%dx%d.pgm", size, size)), size, size)
		for _, strategy := range parallelBenchmarkStrategies {
			for _, workers := range parallelBenchmarkWorkers {
				name := fmt.Sprintf("size=%d/strategy=%s/workers=%d", size, strategy.name, workers)
				b.Run(name, func(b *testing.B) {
					p := Params{Turns: 100, Threads: workers, ImageWidth: size, ImageHeight: size}
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						runParallelBenchmark(p, pixels, strategy)
					}
				})
			}
		}
	}
}

// Reference calculation is deliberately separate from production worker helpers.
func benchmarkReference(pixels []byte, size, turns int) []byte {
	current := append([]byte(nil), pixels...)
	next := make([]byte, len(current))
	for turn := 0; turn < turns; turn++ {
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				count := 0
				for dy := -1; dy <= 1; dy++ {
					for dx := -1; dx <= 1; dx++ {
						if dx == 0 && dy == 0 {
							continue
						}
						if current[((y+dy+size)%size)*size+(x+dx+size)%size] == 255 {
							count++
						}
					}
				}
				index := y*size + x
				next[index] = 0
				if count == 3 || (current[index] == 255 && count == 2) {
					next[index] = 255
				}
			}
		}
		current, next = next, current
	}
	return current
}

func TestParallelBenchmarkCorrectness(t *testing.T) {
	for _, size := range []int{128, 512} {
		pixels := benchmarkImage(t, filepath.Join("..", "images", fmt.Sprintf("%dx%d.pgm", size, size)), size, size)
		expected := benchmarkReference(pixels, size, 100)
		if size == 512 {
			fixture := benchmarkImage(t, filepath.Join("..", "check", "images", "512x512x100.pgm"), size, size)
			if !bytes.Equal(expected, fixture) {
				t.Fatal("independent reference disagrees with supplied 512x512x100 fixture")
			}
		}
		for _, strategy := range parallelBenchmarkStrategies {
			for _, workers := range parallelBenchmarkWorkers {
				t.Run(fmt.Sprintf("size=%d/strategy=%s/workers=%d", size, strategy.name, workers), func(t *testing.T) {
					p := Params{Turns: 100, Threads: workers, ImageWidth: size, ImageHeight: size}
					result := runParallelBenchmark(p, pixels, strategy)
					actual := make([]byte, size*size)
					for _, cell := range result.alive {
						if cell.X < 0 || cell.X >= size || cell.Y < 0 || cell.Y >= size {
							t.Fatalf("invalid final cell: %v", cell)
						}
						actual[cell.Y*size+cell.X] = 255
					}
					if result.turns != 100 || result.finalTurn != 100 || !bytes.Equal(actual, expected) {
						t.Fatalf("incorrect board or completed turns (turn events=%d, final turn=%d)", result.turns, result.finalTurn)
					}
				})
			}
		}
	}
}
