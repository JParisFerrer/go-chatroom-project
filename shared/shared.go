package shared

import (
	"encoding/binary"
	"github.com/golang/protobuf/proto"
	"github.com/jparisferrer/go-chatroom-project/pb"
	"log"
	"net"
)

const (
	ServerPort = 4242
)

func CheckErrorInfo(info string, err error) bool {
	if err != nil {

		log.Printf("%s: %v", info, err)

		return true
	}

	return false
}
func CheckErrorFatal(info string, err error) bool {
	if err != nil {

		log.Fatalf("%s: %v", info, err)

		// never hit
		return true
	}

	return false
}

func readBytes(conn net.Conn, toRead uint64) ([]byte, error) {

	var totalRead uint64 = 0
	var buff []byte = make([]byte, toRead)

	for totalRead < toRead {
		read, err := conn.Read(buff[totalRead:])

		if err != nil {
			if totalRead+uint64(read) == toRead {
				return buff[:], nil

			} else {

				return nil, err
			}
		}

		totalRead += uint64(read)
	}

	return buff[:], nil
}

func ReadProtoMessage(conn net.Conn) (*pb.PBMessage, error) {
	// read 8 little-endian bytes for length, then that many bytes
	lenBytes, err := readBytes(conn, 8)

	if CheckErrorInfo("ReadProtoMessage size", err) {

		return nil, err
	}

	// else read the bytes
	len := binary.LittleEndian.Uint64(lenBytes)

	data, err := readBytes(conn, len)

	if CheckErrorInfo("ReadProtoMessage data", err) {

		return nil, err
	}

	// else we're good, construct the protobuf and return it
	var msg *pb.PBMessage = &pb.PBMessage{}

	err = proto.Unmarshal(data, msg)
	if CheckErrorInfo("ReadProtoMessage parse", err) {

		return nil, err
	}

	return msg, nil
}
