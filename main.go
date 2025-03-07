package main

import (
	"bufio"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/hexagun/common"
)

var addr = "localhost:8100"
var gameId = 111
var playerId = 0

type WebSocketClient struct {
	conn *websocket.Conn
}

func (wsc *WebSocketClient) Connect(addr string) {
	if wsc.conn == nil {
		u := url.URL{Scheme: "ws", Host: addr, Path: "/ws"}
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
	client := &WebSocketClient{}

	//done := make(chan struct{})

	// go func() {
	// 	defer close(done)
	// 	for {
	// 		_, message, err := c.ReadMessage()
	// 		if err != nil {
	// 			log.Println("read:", err)
	// 			return
	// 		}
	// 		log.Printf("recv: %s", message)
	// 	}
	// }()

	for {
		fmt.Print(">: ")
		scanner.Scan()
		input := strings.TrimSpace(scanner.Text())
		fmt.Println(input)

		switch input {
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
						log.Printf("recv: %s", message)
					}
				}()
			}
		case "join":
			if client.IsConnected() {
				joinAction := common.NewJoinAction(gameId, playerId)
				joinMessage := common.NewOutgoingMessage(joinAction.Type, joinAction.GameId, joinAction.PlayerId, nil)
				client.conn.WriteJSON(joinMessage)
			}
		case "start":
			if client.IsConnected() {
				startAction := common.NewStartAction(gameId, playerId)
				startMessage := common.NewOutgoingMessage(startAction.Type, startAction.GameId, startAction.PlayerId, nil)
				client.conn.WriteJSON(startMessage)
			}
		case "play":
			if client.IsConnected() {
				playAction := common.NewPlayerMoveAction(gameId, playerId,
					common.PlayerMovePayload{
						Row: 1,
						Col: 2,
					},
				)
				playMessage := common.NewOutgoingMessage(playAction.Type, playAction.GameId, playAction.PlayerId, playAction.Payload)
				client.conn.WriteJSON(playMessage)
			}
		case "update":
			if client.IsConnected() {
				updateAction := common.NewGameStateUpdateAction(gameId, playerId, common.GameStateUpdatePayload{
					//TODO fill board array
					Board:    [3][3]string{{"X", "O", "X"}, {"O", "X", "O"}, {"O", "X", "O"}},
					NextTurn: "X",
					Winner:   "",
				})
				updateMessage := common.NewOutgoingMessage(updateAction.Type, updateAction.GameId, updateAction.PlayerId, updateAction.Payload)
				client.conn.WriteJSON(updateMessage)
			}
		case "gameover":
			if client.IsConnected() {
				gameoverAction := common.NewGameOverAction(gameId, common.GameOverPayload{
					Winner: "Player1",
					Board:  [3][3]string{{"X", "O", "X"}, {"O", "X", "O"}, {"O", "X", "O"}},
				})
				gameoverMessage := common.NewOutgoingMessage(gameoverAction.Type, gameoverAction.GameId, 0, gameoverAction.Payload)
				client.conn.WriteJSON(gameoverMessage)
			}
		case "error":
			if client.IsConnected() {
				errorAction := common.NewErrorAction("message", common.ErrorPayload{
					Reason: "Failure",
				})
				errorMessage := common.NewOutgoingMessage(errorAction.Type, 0, 0, errorAction.Payload)
				client.conn.WriteJSON(errorMessage)
			}
		case "time":
			fmt.Println("Current time:", time.Now().Format(time.RFC1123))
		case "exit":
			fmt.Println("Exiting program. Goodbye!")
			return
		case "help":
			fmt.Println("Available commands:")
			fmt.Println("  hello  - Prints a greeting")
			fmt.Println("  time   - Shows the current time")
			fmt.Println("  exit   - Exits the program")
			fmt.Println("  help   - Displays this help message")
		default:
			fmt.Println("Unknown command. Type 'help' for a list of commands.")
		}
	}
}
