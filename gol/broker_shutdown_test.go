package gol

import (
	"fmt"
	"net"
	"net/rpc"
	"sync/atomic"
	"testing"
	"time"
)

// ShutdownTestBroker exercises the controller protocol without network sockets.
type ShutdownTestBroker struct {
	stopped  chan struct{}
	saved    atomic.Bool
	killed   atomic.Bool
	shutdown atomic.Bool
}

func (b *ShutdownTestBroker) ProcessStart(_ BrokerArgs, result *bool) error {
	*result = true
	return nil
}

func (b *ShutdownTestBroker) ProcessNextTurn(_ int, result *TurnResult) error {
	<-b.stopped
	*result = TurnResult{Done: true}
	return nil
}

func (b *ShutdownTestBroker) ProcessQuit(_ int, result *QuitResult) error {
	*result = QuitResult{Res: [][]uint8{{255}}}
	close(b.stopped)
	return nil
}

func (b *ShutdownTestBroker) ProcessKill(arg int, result *QuitResult) error {
	b.killed.Store(true)
	return b.ProcessQuit(arg, result)
}

func (b *ShutdownTestBroker) ProcessShutdown(_ int, result *bool) error {
	if !b.saved.Load() {
		return fmt.Errorf("shutdown requested before image output completed")
	}
	b.shutdown.Store(true)
	*result = true
	return nil
}

func TestBrokerQuitAndKillOrdering(t *testing.T) {
	for _, key := range []rune{'q', 'k'} {
		t.Run(string(key), func(t *testing.T) {
			broker := &ShutdownTestBroker{stopped: make(chan struct{})}
			server := rpc.NewServer()
			if err := server.RegisterName("Broker", broker); err != nil {
				t.Fatal(err)
			}
			serverConn, clientConn := net.Pipe()
			defer serverConn.Close()
			go server.ServeConn(serverConn)
			client := rpc.NewClient(clientConn)
			defer client.Close()

			events := make(chan Event, 20)
			keys := make(chan rune, 1)
			input := make(chan uint8, 1)
			input <- 255
			commands := make(chan ioCommand)
			filenames := make(chan string)
			output := make(chan uint8)
			idle := make(chan bool)
			c := distributorChannels{events, commands, idle, filenames, output, input}
			go func() {
				if <-commands != ioOutput {
					return
				}
				<-filenames
				<-output
				if <-commands == ioCheckIdle {
					broker.saved.Store(true)
					idle <- true
				}
			}()
			done := make(chan struct{})
			go func() {
				distributeWithBrokerClient(Params{Turns: 100, Threads: 1, ImageWidth: 1, ImageHeight: 1}, c, keys, client)
				close(done)
			}()
			timeout := time.After(3 * time.Second)
			sawOutput, sawQuit := false, false
			for {
				select {
				case event, ok := <-events:
					if !ok {
						if !sawOutput || !sawQuit || broker.killed.Load() != (key == 'k') || broker.shutdown.Load() != (key == 'k') {
							t.Fatal("missing output, quit, or expected remote shutdown")
						}
						select {
						case <-done:
						case <-timeout:
							t.Fatal("controller did not return")
						}
						return
					}
					switch event := event.(type) {
					case ImageOutputComplete:
						sawOutput = true
					case StateChange:
						if event.NewState == Executing {
							keys <- key
						}
						if event.NewState == Quitting {
							if !sawOutput || (key == 'k' && !broker.shutdown.Load()) {
								t.Fatal("Quitting emitted before output and remote shutdown")
							}
							sawQuit = true
						}
					}
				case <-timeout:
					t.Fatal("controller did not finish quit/kill workflow")
				}
			}
		})
	}
}
