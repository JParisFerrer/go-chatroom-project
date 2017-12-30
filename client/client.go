package main

import (
	"bufio"
	"fmt"
	tm "github.com/buger/goterm"
	"github.com/jparisferrer/go-chatroom-project/pb"
	"github.com/jparisferrer/go-chatroom-project/shared"
	"log"
	"net"
	"os"
	"strings"
	"sync"
)

var chatLog []string
var writeChan chan pb.PBMessage
var netChan chan string

var m_ sync.Mutex

func sendLoop(conn net.Conn) {

	for {
		msg := <-writeChan

		err := shared.SendProtoMessage(&msg, conn)

		shared.CheckErrorFatal("sendLoop", err)
	}
}

func networkLoop(conn net.Conn) {

	go sendLoop(conn)

	// we become read loop

	for {

		msg, err := shared.ReadProtoMessage(conn)
		shared.CheckErrorFatal("networkLoop", err)

		switch msg.Type {

		case pb.PBMessage_NewChat:
			{

				netChan <- msg.Message
			}

		default:
			{

				log.Printf("Unhandled message type: %v", msg.Type)
			}
		}
	}
}

func readTermLoop(responseChan chan string) {

	for {

		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')

		responseChan <- strings.TrimSpace(line)
	}
}

// call WITH the lock
func drawChats() {

	tm.MoveCursor(1, 1)

	resetString := ""
	for i := 0; i < tm.Width(); i++ {

		resetString += " "
	}

	min := len(chatLog) - tm.Height() + 1
	if min < 0 {
		min = 0
	}

	for i := min; i < len(chatLog); i++ {

		tm.Println(resetString)
		tm.MoveCursorUp(1)
		tm.Println(chatLog[i])

	}

	tm.MoveCursor(1, tm.Height())

	tm.Flush()
}

func main() {

	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s <server ip> <username>", os.Args[0])
		os.Exit(1)
	}

	if !shared.ValidateUsername(os.Args[2]) {
		fmt.Fprintf(os.Stderr, "Bad username '%s'", os.Args[2])
		os.Exit(1)
	}

	service := fmt.Sprintf("%v:%d", os.Args[1], shared.ServerPort)

	conn, err := net.Dial("tcp", service)
	shared.CheckErrorFatal(fmt.Sprintf("Bad service '%v'", service), err)

	// send our join message
	smsg := pb.PBMessage{}
	smsg.Type = pb.PBMessage_Join
	smsg.Name = os.Args[2]

	err = shared.SendProtoMessage(&smsg, conn)
	shared.CheckErrorFatal("Error sending hello", err)

	// wait for response of chats so far
	smsg, err = shared.ReadProtoMessage(conn)
	shared.CheckErrorFatal("Error reading initial chats", err)

	for _, chat := range smsg.Messages {

		chatLog = append(chatLog, chat)
	}

	tm.Clear()

	drawChats()

	termChan := make(chan string)
	netChan = make(chan string)
	writeChan = make(chan pb.PBMessage, 1024)

	// start up a goroutine to handle the server
	go networkLoop(conn)
	go readTermLoop(termChan)

	// start up CLI loop
	for {

		select {

		case newChat := <-termChan:
			{
				m_.Lock()
				// send to server
				msg := pb.PBMessage{}
				msg.Type = pb.PBMessage_NewChat
				msg.Message = newChat

				writeChan <- msg

				// clear the bottom line, redraw
				tm.MoveCursor(1, tm.Height())

				resetString := ""
				for i := 0; i < tm.Width(); i++ {

					resetString += " "
				}
				tm.Println(resetString)

				drawChats()

				m_.Unlock()
			}
		case newChat := <-netChan:
			{
				// redraw
				m_.Lock()

				chatLog = append(chatLog, newChat)

				drawChats()

				m_.Unlock()
			}

		}

	}
}
