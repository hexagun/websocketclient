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
		// 	state.PlayerXReady = true
		//state.players[0].token = "x"

		// tr := rules[state.state][0] // Join
		// state.state = tr.State

		fmt.Printf("Player 1 (%s) has connected.\n", state.players[0].name)
	} else if state.players[1].name == "" {
		state.players[1].name = fmt.Sprintf("%d", header.PlayerId)
		//state.players[0].token = "o"
		// 	state.PlayerO = player
		// 	state.PlayerOReady = true
		fmt.Printf("Player 2 (%s) has connected.\n", state.players[1].name)
		// tr := rules[state.state][0] // Join
		// state.state = tr.State
	} else {
		fmt.Printf("Player %s cannot join, both slots are filled.\n", header.PlayerId)
	}
}

func (g *Game) Draw() {
	for _, p := range g.state.players {
		fmt.Println("Player %s", p.name)
	}
	board := g.state.board
	for _, i := range []int{0, 1, 2} {
		fmt.Println("Board [%s,%s,%s]", board[i][0], board[i][1], board[i][2])
	}
}

func updateReducer(state *GameState, action common.Action) {

	//header := action.GetHeader()
	payload := action.GetPayload().(common.GameStateUpdatePayload)
	state.board = payload.Board
	state.activePlayerIndex, _ = strconv.Atoi(payload.NextTurn)
	state.winner = payload.Winner
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
				//startReducer(&g.state, action)
			default:
			}

			g.Draw()
		}
	}
}

func GenerateCustomID() int {
	randomNumber, _ := rand.Int(rand.Reader, big.NewInt(1000000))
	return int(randomNumber.Int64())
}

var addr = "localhost:8100"
var gameId = 111
var playerId = GenerateCustomID()

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

type GameMessageDecoder struct {
}

func (g GameMessageDecoder) Decode(msg *common.IncomingMessage) common.Action {
	var action common.Action

	if msg.GameID != "" {
		id, err := strconv.Atoi(msg.GameID)
		if err != nil {
			// ... handle error
			panic(err)
		}
		gameId = id
	}

	if msg.PlayerID != "" {
		id, err := strconv.Atoi(msg.PlayerID)
		if err != nil {
			// ... handle error
			panic(err)
		}
		playerId = id
	}

	switch msg.Type {
	case "gamestateupdate":
		var updatePayload struct {
			Board    [3][3]string `json:"board"`
			NextTurn string       `json:"nextturn"`
			Winner   string       `json:"winner"`
		}
		if err := json.Unmarshal(msg.Payload, &updatePayload); err == nil {
			//fmt.Printf("Player made a move at row %d, col %d\n", movePayload.Row, movePayload.Col)
			action = common.NewGameStateUpdateAction(gameId, playerId, common.GameStateUpdatePayload{
				Board:    updatePayload.Board,
				NextTurn: updatePayload.NextTurn,
				Winner:   updatePayload.Winner,
			})
		}
	//case "gameover":
	//case "error":
	case "start":
		action = common.NewStartAction(gameId, playerId)
	case "join":
		action = common.NewJoinAction(gameId, playerId)
	// case "leave":
	// 	action = common.NewLeaveAction(gameId, playerId)
	case "move":
		var movePayload struct {
			Row int `json:"row"`
			Col int `json:"col"`
		}
		if err := json.Unmarshal(msg.Payload, &movePayload); err == nil {
			fmt.Printf("Player made a move at row %d, col %d\n", movePayload.Row, movePayload.Col)
			action = common.NewPlayerMoveAction(gameId, playerId, common.PlayerMovePayload{
				Row: movePayload.Row,
				Col: movePayload.Col,
			})
		}

	default:
		fmt.Println("Not Decoding stuff")
	}

	return action
	//game.Dispatch(action)
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
						decoder := GameMessageDecoder{}
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
