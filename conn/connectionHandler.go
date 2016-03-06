// Copyright 2015 ThoughtWorks, Inc.

// This file is part of Gauge.

// Gauge is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// Gauge is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with Gauge.  If not, see <http://www.gnu.org/licenses/>.

package conn

import (
	"bytes"
	"fmt"
	"net"
	"time"

	"google.golang.org/grpc"

	"github.com/getgauge/gauge/logger"
	"github.com/golang/protobuf/proto"
)

type messageHandler interface {
	MessageBytesReceived([]byte, net.Conn)
}

type dataHandlerFn func(*GaugeConnectionHandler, []byte)

type GaugeConnectionHandler struct {
	tcpListener    *net.TCPListener
	V2PortListener *net.TCPListener
	messageHandler messageHandler
	GRPCServer     *grpc.Server
}

func NewGaugeConnectionHandler(port, v2port int, messageHandler messageHandler) (*GaugeConnectionHandler, error) {
	// port = 0 means GO will find a unused port
	s := grpc.NewServer()
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{Port: port})
	if err != nil {
		return nil, err
	}
	v2portlistener, err := net.ListenTCP("tcp", &net.TCPAddr{Port: v2port})
	if err != nil {
		return nil, err
	}
	return &GaugeConnectionHandler{tcpListener: listener, messageHandler: messageHandler, GRPCServer: s, V2PortListener: v2portlistener}, nil
}

func (h *GaugeConnectionHandler) ServeGRPCServer() {
	err := h.GRPCServer.Serve(h.V2PortListener)
	if err != nil {
		logger.Fatalf("Failed to serve GRPC Server. %s", err.Error())
	}
}

func (h *GaugeConnectionHandler) AcceptConnection(connectionTimeOut time.Duration, errChannel chan error) (net.Conn, error) {
	connectionChannel := make(chan net.Conn)

	go func() {
		connection, err := h.tcpListener.Accept()
		if err != nil {
			errChannel <- err
		}
		if connection != nil {
			connectionChannel <- connection
		}
	}()

	select {
	case err := <-errChannel:
		return nil, err
	case conn := <-connectionChannel:
		if h.messageHandler != nil {
			go h.handleConnectionMessages(conn)
		}
		return conn, nil
	case <-time.After(connectionTimeOut):
		return nil, fmt.Errorf("Timed out connecting to %v", h.tcpListener.Addr())
	}
}

func (h *GaugeConnectionHandler) acceptConnectionWithoutTimeout() (net.Conn, error) {
	errChannel := make(chan error)
	connectionChannel := make(chan net.Conn)

	go func() {
		connection, err := h.tcpListener.Accept()
		if err != nil {
			errChannel <- err
		}
		if connection != nil {
			connectionChannel <- connection
		}
	}()

	select {
	case err := <-errChannel:
		return nil, err
	case conn := <-connectionChannel:
		if h.messageHandler != nil {
			go h.handleConnectionMessages(conn)
		}
		return conn, nil
	}
}

func (h *GaugeConnectionHandler) handleConnectionMessages(conn net.Conn) {
	buffer := new(bytes.Buffer)
	data := make([]byte, 8192)
	for {
		n, err := conn.Read(data)
		if err != nil {
			conn.Close()
			logger.APILog.Info("Closing connection [%s] cause: %s", h.ConnectionPortNumber(), err.Error())
			return
		}

		buffer.Write(data[0:n])
		h.processMessage(buffer, conn)
	}
}

func (h *GaugeConnectionHandler) processMessage(buffer *bytes.Buffer, conn net.Conn) {
	for {
		messageLength, bytesRead := proto.DecodeVarint(buffer.Bytes())
		if messageLength > 0 && messageLength < uint64(buffer.Len()) {
			messageBoundary := int(messageLength) + bytesRead
			receivedBytes := buffer.Bytes()[bytesRead : messageLength+uint64(bytesRead)]
			h.messageHandler.MessageBytesReceived(receivedBytes, conn)
			buffer.Next(messageBoundary)
			if buffer.Len() == 0 {
				return
			}
		} else {
			return
		}
	}
}

// HandleMultipleConnections accepts multiple connections and Handler responds to incoming messages
func (h *GaugeConnectionHandler) HandleMultipleConnections() {
	for {
		h.acceptConnectionWithoutTimeout()
	}
}

func (h *GaugeConnectionHandler) ConnectionPortNumber() int {
	if h.tcpListener != nil {
		return h.tcpListener.Addr().(*net.TCPAddr).Port
	}
	return 0
}

func (h *GaugeConnectionHandler) APIV2PortNumber() int {
	if h.V2PortListener != nil {
		return h.V2PortListener.Addr().(*net.TCPAddr).Port
	}
	return 0
}
