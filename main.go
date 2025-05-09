package main

import (
	"bufio"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/gorilla/websocket"

	"github.com/hexagun/common"
)

type Player struct {
	id    int
	name  string
	token string
}

type GameState struct {
	// state             State
	board             [3][3]string
	players           [2]Player
	activePlayerIndex int
	winner            string
	draw              bool
}

type Game struct {
	state         GameState
	actionChannel chan common.Action
	//pool          *websocket.Pool // Seems wrong
}

func GenerateCustomID() int {
	randomNumber, _ := rand.Int(rand.Reader, big.NewInt(1000000))
	return int(randomNumber.Int64())
}

var addr = "localhost:8100"
var gameId = 111
var playerId = GenerateCustomID()

func NewGame() *Game {
	return &Game{
		state: GameState{
			//state: InitialState,
		},
		actionChannel: make(chan common.Action),
	}
}

func (g *Game) Dispatch(action common.Action) {
	g.actionChannel <- action
}

func joinReducer(state *GameState, action common.Action) {
	header := action.GetHeader()

	if state.players[0].name == "" {
		state.players[0].name = fmt.Sprintf("%d", header.PlayerId)

		// tr := rules[state.state][0] // Join
		// state.state = tr.State

		fmt.Printf("Player 1 (%s) has connected.\n", state.players[0].name)
	} else if state.players[1].name == "" {
		state.players[1].name = fmt.Sprintf("%d", header.PlayerId)

		// tr := rules[state.state][0] // Join
		// state.state = tr.State

		fmt.Printf("Player 2 (%s) has connected.\n", state.players[1].name)

	} else {
		fmt.Printf("Player %s cannot join, both slots are filled.\n", header.PlayerId)
	}
}

func (g *Game) Draw() {
	noToken := "-"
	var token string

	for i, p := range g.state.players {
		if p.token == "" {
			token = noToken
		} else {
			token = p.token
		}

		turnToken := ""
		if g.state.activePlayerIndex == i {
			turnToken = ">:"
		}

		playerStringStr := fmt.Sprintf("%s Player %s (%s)", turnToken, p.name, token)
		fmt.Println(playerStringStr)
	}
	board := g.state.board
	for _, i := range []int{0, 1, 2} {
		row := [3]string{"-", "-", "-"}
		var tile string
		for _, j := range []int{0, 1, 2} {
			if board[i][j] == "" {
				tile = noToken
			} else {
				tile = board[i][j]
			}
			row[j] = tile
		}

		boardLineStringStr := fmt.Sprintf("Board [%s,%s,%s]", row[0], row[1], row[2])
		fmt.Println(boardLineStringStr)
	}
	fmt.Print(">: ")
}

func updateReducer(state *GameState, action common.Action) {

	//header := action.GetHeader()
	payload := action.GetPayload().(common.GameStateUpdatePayload)
	state.board = payload.Board

	if payload.NextTurn == state.players[0].name {
		state.activePlayerIndex = 0
	} else {
		state.activePlayerIndex = 1
	}

	state.winner = payload.Winner
}

// func Reduce[T any, R any](input []T, init R, f func(R, T) R) R {
// 	result := init
// 	for _, v := range input {
// 		result = f(result, v)
// 	}
// 	return result
// }

func startReducer(state *GameState, action common.Action) {

	// YourToken  string //`json:"yourToken"`
	// OpponentID string //`json:"opponentId"`
	// FirstTurn  string //`json:"firstTurn"` // could also be a player ID

	// header := action.GetHeader()
	payload := action.GetPayload().(common.StartPayload)

	state.players[0].token = payload.YourToken
	state.players[0].name = strconv.Itoa(action.GetHeader().PlayerId)

	if payload.YourToken == "x" {
		state.players[1].token = "o"
	} else {
		state.players[1].token = "x"
	}

	state.players[1].name = payload.OpponentID

	if payload.FirstTurn == state.players[0].name {
		state.activePlayerIndex = 0
	} else {
		state.activePlayerIndex = 1
	}
}

func (g *Game) GameLoop() {
	for {
		select {
		case action := <-g.actionChannel:

			header := action.GetHeader()
			//payload := action.GetPayload()

			switch header.Type {
			case common.GameStateUpdate:
				updateReducer(&g.state, action)
			case common.Join:
				joinReducer(&g.state, action)
			case common.Start:
				startReducer(&g.state, action)
			default:
			}

			g.Draw()
		}
	}
}

type WebSocketClient struct {
	conn *websocket.Conn
}

func (wsc *WebSocketClient) Connect(addr string) {
	if wsc.conn == nil {
		u := url.URL{Scheme: "ws", Host: addr, Path: "/ws"}

		q := u.Query()
		q.Set("id", strconv.Itoa(playerId))
		u.RawQuery = q.Encode()

		fmt.Println(u)
		c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
		if err != nil {
			log.Fatal("dial:", err)
		}
		wsc.conn = c
	} else {
		panic("Already connected.")
	}
}

func (wsc *WebSocketClient) IsConnected() bool {
	return wsc.conn != nil
}

func (wsc *WebSocketClient) Close() {
	if wsc.conn != nil {
		wsc.conn.Close()
	} else {
		panic("Already disconnected.")
	}
}

func main() {

	scanner := bufio.NewScanner(os.Stdin)

	game := NewGame()
	go game.GameLoop()

	client := &WebSocketClient{}

	for {
		fmt.Print(">: ")
		scanner.Scan()
		inputText := strings.TrimSpace(scanner.Text())
		input := strings.Split(inputText, " ")
		fmt.Println()

		switch input[0] {
		case "connect":
			if !client.IsConnected() {
				client.Connect(addr)
				defer client.Close()

				go func() {
					//defer close(done)
					for {
						_, message, err := client.conn.ReadMessage()
						if err != nil {
							log.Println("read:", err)
							return
						}

						var msg common.IncomingMessage

						if err := json.Unmarshal(message, &msg); err != nil {
							log.Println("JSON Unmarshal error:", err)
							continue
						}

						fmt.Printf("Message Decoded: %+v\n", msg)
						decoder := common.MessageDecoder{}
						action := decoder.Decode(&msg)
						game.Dispatch(action)
						log.Printf("recv: %s", message)
					}
				}()
			}
		case "play":
			if client.IsConnected() {
				if len(input) > 2 {
					row, errRow := strconv.Atoi(input[1])
					col, errCol := strconv.Atoi(input[2])
					if errRow != nil || errCol != nil {
						log.Println("Missing row or col")
						break
					}

					playAction := common.NewPlayerMoveAction(gameId, playerId,
						common.PlayerMovePayload{
							Row: row,
							Col: col,
						},
					)
					header := playAction.GetHeader()
					payload := playAction.GetPayload()
					playMessage := common.NewOutgoingMessage(header.Type, header.GameId, header.PlayerId, payload)
					client.conn.WriteJSON(playMessage)
				}

			}
		case "update":
			if client.IsConnected() {
				game.Draw()
			}
		case "error":
			if client.IsConnected() {
				errorAction := common.NewErrorAction("message", "Failure")
				// ,common.ErrorPayload{
				// 	Reason: "Failure",
				// })
				header := errorAction.GetHeader()
				payload := errorAction.GetPayload()
				errorMessage := common.NewOutgoingMessage(header.Type, 0, 0, payload)
				client.conn.WriteJSON(errorMessage)
			}
		case "exit":
			fmt.Println("Exiting program. Goodbye!")
			return
		case "help":
			fmt.Println("Available commands:")
			fmt.Println("  play  - Prints a greeting")
			fmt.Println("  time   - Shows the current time")
			fmt.Println("  exit   - Exits the program")
			fmt.Println("  help   - Displays this help message")
		default:
			fmt.Println("Unknown command. Type 'help' for a list of commands.")
		}
	}
}
