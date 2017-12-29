package main

import (
	"fmt"
	"github.com/jparisferrer/go-chatroom-project/pb"
	"github.com/jparisferrer/go-chatroom-project/shared"
	"net"
	"sync"
)

// global state mutex
var m_ sync.Mutex

// will have a read and write channel for each client
var clientReadChannels map[uint64]chan pb.PBMessage
var clientWriteChannels map[uint64]chan pb.PBMessage
var clientSockets map[uint64]net.Conn

// next client id to assign
var nextClientId uint64

// our "real" state
var chatLog []string              // log of all chat messages in the room
var clientNames map[uint64]string // usernames of all the clients

// blocking read loop on their socket
// exits itself when the socket is closed
func readClient(id uint64) {
	m_.Lock()
	con, ok := clientSockets[id]
	m_.Unlock()

	if !ok {

		return
	}

	// read loop

}

// per-client routine, read/write loop on their channels
// use a for-select
func handleClient(id uint64) {
	m_.Lock()
	readChan, ok := clientReadChannels[id]

	if !ok {

		return
	}

	writeChan, ok := clientWriteChannels[id]

	if !ok {

		return
	}

	m_.Unlock()

	// start off a go routine to blocking read off the socket
	go readClient(id)

	// for-select on our channels
	for {

		select {

		case msg := <-readChan:
			{
				m_.Lock()

				m_.Unlock()
			}

		case msg := <-writeChan:
			{
				m_.Lock()

				m_.Unlock()
			}

		}

	}
}

// just a small helper to read initial message from the new connection and parse
// will be run as a goroutine so that a slow/malicious client can't stop us from accepting
// new connections
func handleNewClient(con net.Conn) {

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
