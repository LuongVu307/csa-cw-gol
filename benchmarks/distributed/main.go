// Command distributed measures the Go broker protocol without SDL or disk output.
package main

import (
	"bytes"
	"encoding/csv"
	"flag"
	"fmt"
	"net/rpc"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"uk.ac.bris.cs/gameoflife/util"
)

type params struct{ Turns, Threads, ImageWidth, ImageHeight int }
type startArgs struct {
	P     params
	World [][]uint8
}
type turnResult struct {
	Turn  int
	Cells []util.Cell
	Done  bool
	Res   [][]uint8
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func readPGM(path string) (int, int, []byte) {
	data, err := os.ReadFile(path)
	check(err)
	parts := bytes.SplitN(data, []byte{'\n'}, 4)
	if len(parts) != 4 || string(parts[0]) != "P5" || string(parts[2]) != "255" {
		panic("expected supplied P5 fixture with three-line header: " + path)
	}
	var width, height int
	_, err = fmt.Sscanf(string(parts[1]), "%d %d", &width, &height)
	check(err)
	if width <= 0 || height <= 0 || len(parts[3]) != width*height {
		panic("invalid PGM dimensions or pixel count: " + path)
	}
	return width, height, parts[3]
}

func main() {
	workers := flag.Int("workers", 0, "Actual configured remote worker count (at least 2); this label does not configure the broker")
	turns := flag.Int("turns", 100, "Number of turns")
	count := flag.Int("count", 3, "Independent samples")
	setup := flag.String("setup", "", "Required description of machines, network and CPU limits")
	output := flag.String("output", "", "Required new CSV path; existing files are never overwritten")
	input := flag.String("input", "images/512x512.pgm", "Initial PGM")
	reference := flag.String("reference", "check/images/512x512x100.pgm", "Expected final PGM for these turns")
	address := flag.String("broker", "", "Broker host:port; default reads brokerURL.txt")
	flag.Parse()
	if *workers < 2 || *turns < 1 || *count < 1 || *setup == "" || *output == "" {
		panic("provide -workers >= 2, -setup, -output, and positive turns/count")
	}
	if *address == "" {
		data, err := os.ReadFile("brokerURL.txt")
		check(err)
		*address = strings.TrimSpace(string(data))
	}
	width, height, pixels := readPGM(*input)
	refWidth, refHeight, expected := readPGM(*reference)
	if width != refWidth || height != refHeight {
		panic("reference dimensions differ from input")
	}
	file, err := os.OpenFile(*output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	check(err)
	defer file.Close()
	writer := csv.NewWriter(file)
	check(writer.Write([]string{"utc", "setup", "broker", "workers", "width", "height", "turns", "sample", "milliseconds", "go_version", "gomaxprocs", "validation"}))
	writer.Flush()
	check(writer.Error())
	fmt.Fprintf(os.Stderr, "Verify broker workerURLs.txt has exactly %d worker addresses. This command does not change it.\n", *workers)
	for sample := 1; sample <= *count; sample++ {
		world := make([][]uint8, height)
		for y := range world {
			world[y] = append([]uint8(nil), pixels[y*width:(y+1)*width]...)
		}
		replay := append([]byte(nil), pixels...)
		changes := make([][]util.Cell, 0, *turns)
		// Time connection establishment, initial transfer, all turn RPCs and final transfer.
		started := time.Now()
		client, err := rpc.Dial("tcp", *address)
		check(err)
		var accepted bool
		check(client.Call("Broker.ProcessStart", startArgs{P: params{*turns, *workers, width, height}, World: world}, &accepted))
		if !accepted {
			panic("broker did not accept simulation")
		}
		batches := 0
		var final turnResult
		for {
			var result turnResult
			check(client.Call("Broker.ProcessNextTurn", 0, &result))
			if result.Done {
				final = result
				break
			}
			batches++
			if result.Turn != batches {
				panic("missing or out-of-order turn")
			}
			// Retain decoded changes; replay and fixture comparison follow timing.
			changes = append(changes, result.Cells)
		}
		elapsed := time.Since(started)
		check(client.Close())
		if batches != *turns || final.Turn != *turns || len(final.Res) != height {
			panic("incorrect completed turn count or final dimensions")
		}
		for _, cells := range changes {
			for _, cell := range cells {
				if cell.X < 0 || cell.X >= width || cell.Y < 0 || cell.Y >= height {
					panic("invalid flipped-cell coordinate")
				}
				replay[cell.Y*width+cell.X] ^= 255
			}
		}
		actual := make([]byte, 0, len(pixels))
		for _, row := range final.Res {
			if len(row) != width {
				panic("incorrect final row width")
			}
			actual = append(actual, row...)
		}
		if !bytes.Equal(actual, expected) || !bytes.Equal(replay, expected) {
			panic("final board or replayed turn stream differs from reference")
		}
		check(writer.Write([]string{time.Now().UTC().Format(time.RFC3339), *setup, *address, strconv.Itoa(*workers), strconv.Itoa(width), strconv.Itoa(height), strconv.Itoa(*turns), strconv.Itoa(sample), fmt.Sprintf("%.6f", float64(elapsed)/float64(time.Millisecond)), runtime.Version(), strconv.Itoa(runtime.GOMAXPROCS(0)), "PASS"}))
		writer.Flush()
		check(writer.Error())
		fmt.Fprintf(os.Stderr, "workers=%d sample=%d %.3f ms PASS\n", *workers, sample, float64(elapsed)/float64(time.Millisecond))
	}
}
