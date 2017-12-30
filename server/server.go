package main

import (
	"fmt"
	"github.com/jparisferrer/go-chatroom-project/pb"
	"github.com/jparisferrer/go-chatroom-project/shared"
	"log"
	"net"
	"sync"
	"time"
)

// global state mutex
var m_ sync.Mutex

// will have a read and write channel for each client
var clientReadChannels map[uint64]chan pb.PBMessage
var clientWriteChannels map[uint64]chan pb.PBMessage
var clientErrorChannels map[uint64]chan error
var clientSockets map[uint64]net.Conn

// next client id to assign
var nextClientId uint64

// our "real" state
var chatLog []string              // log of all chat messages in the room
var clientNames map[uint64]string // usernames of all the clients

// queues the message to all current clients
// LOCK m_ BEFORE YOU CALL
func queueToAll(msg pb.PBMessage) {

	for _, ch := range clientWriteChannels {

		ch <- msg
	}

}

// queues the message to all current clients
// LOCK m_ BEFORE YOU CALL
func queueToAllBut(msg pb.PBMessage, id uint64) {

	for k, ch := range clientWriteChannels {

		if k != id {
			ch <- msg
		}
	}

}

// blocking read loop on their socket
// exits itself when the socket is closed
func readClient(id uint64) {
	m_.Lock()
	con, ok := clientSockets[id]
	errChan, ok2 := clientErrorChannels[id]
	readChan, ok3 := clientReadChannels[id]
	m_.Unlock()

	if !ok || !ok2 || !ok3 {

		return
	}

	defer func() {
		// tried to send on a closed channel, ie errChan
		// this will happen if for example the sendClient goroutine
		// triggered the master thru errChan

		recover()
	}()

	// read loop
	for {

		msg, err := shared.ReadProtoMessage(con)

		// if error, send it up to the main go routine for this client which can then exit
		if shared.CheckErrorInfo("readClient "+string(id), err) {
			errChan <- err

			return
		}
		// else good message, forward it along
		readChan <- msg
	}

}

// send loop for this client
func sendClient(id uint64, echan chan bool) {
	m_.Lock()
	con, ok := clientSockets[id]
	errChan, ok2 := clientErrorChannels[id]
	sendChan, ok3 := clientWriteChannels[id]
	m_.Unlock()

	if !ok || !ok2 || !ok3 {

		return
	}

	defer func() {
		// tried to send on a closed channel, ie errChan
		// this will happen if for example the readClient goroutine
		// triggered the master thru errChan

		recover()
	}()

	for {
		select {

		case msg := <-sendChan:
			{
				// message to send

				err := shared.SendProtoMessage(&msg, con)

				if shared.CheckErrorInfo("sendClient"+string(id), err) {

					errChan <- err
					return
				}
			}

		case <-echan:
			{
				// told by master goroutine to quit

				return
			}

		}

	}

}

// per-client routine, read/write loop on their channels
// use a for-select
func handleClient(id uint64) {
	m_.Lock()
	readChan, ok := clientReadChannels[id]

	if !ok {

		m_.Unlock()
		return
	}

	writeChan, ok := clientWriteChannels[id]

	if !ok {

		m_.Unlock()
		return
	}

	errChan, ok := clientErrorChannels[id]

	if !ok {

		m_.Unlock()
		return
	}

	var echan chan bool = make(chan bool)

	// start off a go routine to blocking read off the socket
	go readClient(id)
	// start off a go routine to send off the socket
	go sendClient(id, echan)

	// add a chat saying we've joined
	var smsg pb.PBMessage = pb.PBMessage{}
	var joinMesg string = fmt.Sprintf("[%v][Server] '%v' has joined.", time.Now(), clientNames[id])
	smsg.Type = pb.PBMessage_NewChat
	smsg.Message = joinMesg
	chatLog = append(chatLog, joinMesg)

	queueToAllBut(smsg, id)

	// queue off the current chats to this client, with the lock
	// so that they won't miss any that users are currently writing
	smsg = pb.PBMessage{}
	smsg.Type = pb.PBMessage_StartingState
	smsg.Messages = make([]string, len(chatLog))

	for _, chat := range chatLog {
		smsg.Messages = append(smsg.Messages, chat)
	}

	writeChan <- smsg

	m_.Unlock()

	// for-select on our channels
	for {

		select {

		case msg := <-readChan:
			{
				m_.Lock()

				switch msg.Type {

				case pb.PBMessage_NewChat:
					{
						var newString string
						newString = fmt.Sprintf("[%v][%s] %s", time.Now(), clientNames[id], msg.Message)

						chatLog = append(chatLog, newString)

						msg.Message = newString
						// queue up to all clients
						queueToAll(msg)
					}
				case pb.PBMessage_Leave:
					{
						close(errChan)

						fmt.Printf("Client %v (id %v) left voluntarily \n", clientSockets[id], id)

						clientSockets[id].Close()
						delete(clientSockets, id)
						delete(clientReadChannels, id)
						delete(clientWriteChannels, id)
						delete(clientErrorChannels, id)

						var leaveMesg string = fmt.Sprintf("[%v][Server] '%s' has left.", time.Now(), clientNames[id])
						msg.Type = pb.PBMessage_NewChat // maybe change later to a Leave
						msg.Message = leaveMesg

						queueToAll(msg)

						chatLog = append(chatLog, leaveMesg)

						m_.Unlock()

						echan <- true
						return
					}
				}

				m_.Unlock()
			}

		case err := <-errChan:
			{
				m_.Lock()

				log.Printf("Got error: %v", err)

				clientSockets[id].Close()
				close(errChan)

				delete(clientSockets, id)
				delete(clientReadChannels, id)
				delete(clientWriteChannels, id)
				delete(clientErrorChannels, id)

				var msg pb.PBMessage

				var leaveMesg string = fmt.Sprintf("[%v][Server] '%s' has left.", time.Now(), clientNames[id])
				msg.Type = pb.PBMessage_NewChat // maybe change later to a Leave
				msg.Message = leaveMesg

				queueToAll(msg)

				chatLog = append(chatLog, leaveMesg)

				m_.Unlock()

				echan <- true
			}

		}

	}
}

// just a small helper to read initial message from the new connection and parse
// will be run as a goroutine so that a slow/malicious client can't stop us from accepting
// new connections
func handleNewClient(con net.Conn) {
	msg, err := shared.ReadProtoMessage(con)

	if shared.CheckErrorInfo("handleNewClient", err) {

		log.Printf("Error handling connection from %v", con)

		con.Close()

		return
	}
	// else we're good, parse the messagestart up the goroutine
	log.Printf("Starting session with %v", con)

	if msg.Type == pb.PBMessage_Join {
		m_.Lock()

		id := nextClientId
		nextClientId++

		clientSockets[id] = con
		clientReadChannels[id] = make(chan pb.PBMessage)
		clientWriteChannels[id] = make(chan pb.PBMessage)
		clientErrorChannels[id] = make(chan error)

		clientNames[id] = msg.Name

		m_.Unlock()

		go handleClient(id)
	} else {

		log.Printf("%v had bad message type %v ", msg.Type)

		con.Close()
		return
	}

}

// Accept loop
func main() {

	service := fmt.Sprintf(":%d", shared.ServerPort)
	listener, err := net.Listen("tcp", service)

	shared.CheckErrorFatal("main Listen", err)

	for {
		conn, err := listener.Accept()

		if shared.CheckErrorInfo("accept", err) {

			continue
		}

		// else we're good
		go handleNewClient(conn)

	}

}
