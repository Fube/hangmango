package main

import (
	"bytes"
	"fmt"
	"math"
	"net"
	"sockets/hangman"
	"sockets/multiplayer"
	"sockets/terminator"
	"sync"
	"time"
)

type MultiplayerHangman struct {
	hangman.Hangman
	multiplayer.Server
	guessMutex sync.Mutex
	turnCount  int
}

func CreateMultiplayerHangman(h hangman.Hangman, s multiplayer.Server) *MultiplayerHangman {
	return &MultiplayerHangman{
		guessMutex: sync.Mutex{},
		turnCount:  0,
		Hangman:    h,
		Server:     s,
	}
}

func (mh *MultiplayerHangman) Guess(g byte) (string, error) {
	mh.guessMutex.Lock()
	defer mh.guessMutex.Unlock()

	nextState, err := mh.Hangman.Guess(g)
	if err == nil {
		mh.Broadcast(&multiplayer.Message{
			Content: []byte(nextState + "\n"),
			Source:  mh,
			Type:    multiplayer.Normal,
		})
		mh.turnCount++
	}

	return nextState, err
}

func (mh *MultiplayerHangman) IsTurnOf(c multiplayer.Client) bool {
	r := false

	mh.DoWithClients(func(cs []multiplayer.Client) {
		r = cs[mh.turnCount%len(cs)].GetId() == c.GetId()
	})

	return r
}

// --

func main() {
	game := hangman.New()

	var wg sync.WaitGroup

	server, err := net.Listen("tcp", "0.0.0.0:9191")
	if err != nil {
		panic(err)
	}
	defer server.Close()

	multiplayerHangman := CreateMultiplayerHangman(game, multiplayer.CreateServer(server))

	for {

		connection, err := server.Accept()
		if err != nil {
			fmt.Println(err.Error())
			break
		}
		wg.Add(1)

		client := multiplayerHangman.AddClient(connection)
		client.GetInput() <- &multiplayer.Message{Content: []byte(multiplayerHangman.GetCurrentState() + "\n"), Type: multiplayer.Normal, Source: multiplayerHangman}
		go handle(client, &wg, multiplayerHangman)
	}

	wg.Wait()
}

func handle(client multiplayer.Client, wg *sync.WaitGroup, multiplayerHangman *MultiplayerHangman) {
	term := terminator.New(client)
	var errMsg []byte = nil

	msg := "It's your turn"
	colors := []terminator.Color{terminator.Blue, terminator.Green, terminator.Red, terminator.Orange}
	shift := 0
	turnLine := terminator.LineFromGenerator(func() []byte {
		var buffer bytes.Buffer

		if !multiplayerHangman.IsTurnOf(client) {
			buffer.WriteString(string(terminator.Red))
			buffer.WriteString("It is NOT your turn")
			buffer.WriteString(string(terminator.Reset))
			return buffer.Bytes()
		}

		buffer.WriteString(string(colors[int(math.Abs(float64(shift-1)))%len(colors)]))
		for i, c := range msg {
			buffer.WriteRune(c)
			buffer.WriteString(string(colors[(i+shift)%len(colors)]))
		}

		buffer.WriteString(string(terminator.Reset))

		shift++

		return buffer.Bytes()
	})

	stateLine := terminator.LineFromGenerator(func() []byte {
		return []byte(multiplayerHangman.GetCurrentState())
	})

	errLine := terminator.LineFromGenerator(func() []byte {
		if errMsg == nil {
			return nil
		}

		var buffer bytes.Buffer

		buffer.WriteString(string(terminator.Red))
		buffer.Write(errMsg)
		buffer.WriteString(string(terminator.Reset))

		return buffer.Bytes()
	})

	gameOverLine := terminator.AnimatedLineFromGenerator(func () []byte {
		if multiplayerHangman.IsOver() {
			return []byte("Game is over")
		}

		return nil
	}, colors)

	term.AddLine(stateLine)
	term.AddLine(turnLine)
	term.AddLine(gameOverLine)
	term.AddLine(errLine)
	term.AddLine(term.CreateInputLine('>'))

	term.HideLine(errLine)
	term.HideLine(gameOverLine)

	go func() {
		for {
			if err := term.Draw(); err != nil {
				fmt.Println(err.Error())
				return
			}

			time.Sleep(time.Millisecond * 200)
		}
	}()

	go func() {
		for range client.GetInput() {
			if multiplayerHangman.IsOver() {
				term.HideLine(turnLine)
				term.ShowLine(gameOverLine)
				term.HideLine(errLine)
			}
		}
	}()

	for {
		buffer := make([]byte, 16)
		read, err := client.Read(buffer)

		if err != nil {
			client.Close()
			wg.Done()
			return
		}

		errMsg = nil
		term.HideLine(errLine)

		term.HadInput()

		if multiplayerHangman.IsOver() || !multiplayerHangman.IsTurnOf(client) {
			continue
		}

		if read > 0 {
			_, err := multiplayerHangman.Guess(buffer[0])
			if err != nil {
				errMsg = []byte(err.Error())
				term.ShowLine(errLine)
			}
		}
	}
}
