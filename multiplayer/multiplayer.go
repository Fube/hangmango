package multiplayer

import (
	"net"
	"sync"
)

type MessageType int
const (
	Error MessageType = iota
	Normal
)

type Message struct {
	Content []byte
	Source interface{}
	Type MessageType
}

type Broadcaster interface {
	Broadcast(message *Message)
}

// --

type server struct {
	net.Listener
	connectedClients []Client
	clientMutex sync.Mutex
}

type Server interface {
	Broadcaster
	net.Listener
	AddClient(c net.Conn) Client
	RemoveClient(c Client)
	DoWithClients(op func([]Client) ) 
}

func CreateServer(l net.Listener) Server {
	return &server{
		connectedClients: make([]Client, 0),
		clientMutex: sync.Mutex{},
		Listener: l,
	}
}

func (s *server) Broadcast(message *Message) {
	for _, c := range s.connectedClients {
		select {
		case c.GetInput()<-message: continue;
		default: continue;
		}
	}
}

func (s *server) AddClient(c net.Conn) Client {
	s.clientMutex.Lock()
	defer s.clientMutex.Unlock()

	
	client := createClient(c, s, len(s.connectedClients))
	s.connectedClients = append(s.connectedClients, client)

	return client
}

func (s *server) RemoveClient(c Client) {
	s.clientMutex.Lock()
	defer s.clientMutex.Unlock()

	if len(s.connectedClients) == 1 {
		s.connectedClients = nil
		return;
	}

	s.connectedClients = append(s.connectedClients[:c.GetId()], s.connectedClients[c.GetId()+1:]...)
}

func (s *server) DoWithClients(op func([]Client)) {
	s.clientMutex.Lock()
	defer s.clientMutex.Unlock()

	op(s.connectedClients)
}

// --
type client struct {
	net.Conn
	In chan *Message
	id int
	owner *server
}

type Client interface {
	net.Conn
	GetInput() chan *Message
	GetId() int
	CreateMessage(content []byte) *Message
	CreateErrorMessage(content []byte) *Message
}

func createClient(c net.Conn, s *server, id int) *client {
	return &client{
		Conn: c,
		owner: s,
		id: id,
		In: make(chan *Message, 16),
	}
}

func (c *client) Close() error {
	c.owner.RemoveClient(c)

	return c.Conn.Close()
}

func (c *client) GetId() int {
	return c.id
}

func (c *client) CreateMessage(content []byte) *Message {
	return &Message {
		Content: content,
		Source: c,
		Type: Normal,
	}
}


func (c *client) CreateErrorMessage(content []byte) *Message {
	return &Message {
		Content: content,
		Source: c,
		Type: Error,
	}
}

func (c *client) GetInput() chan *Message {
	return c.In
}