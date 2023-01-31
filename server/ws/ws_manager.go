package ws

import (
	"context"
	"log"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/juju/errors"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

type WSManager struct {
	connections       map[*websocket.Conn]struct{}
	connSubscriptions map[*websocket.Conn]uuid.UUID
	subscriptionConns map[uuid.UUID]map[*websocket.Conn]struct{}
	connectionsMtx    *sync.RWMutex
}

func NewWSManager() *WSManager {
	return &WSManager{
		connections:       map[*websocket.Conn]struct{}{},
		connSubscriptions: map[*websocket.Conn]uuid.UUID{},
		subscriptionConns: map[uuid.UUID]map[*websocket.Conn]struct{}{},
		connectionsMtx:    new(sync.RWMutex),
	}
}

func (wsm *WSManager) AddConnection(c *gin.Context) error {
	conn, err := websocket.Accept(c.Writer, c.Request, nil)
	if err != nil {
		return errors.Trace(err)
	}

	wsm.connectionsMtx.Lock()
	wsm.connections[conn] = struct{}{}
	wsm.connectionsMtx.Unlock()

	go wsm.listen(conn)

	log.Println("Add connection", wsm.NumConns())

	return nil
}

func (wsm *WSManager) subscribe(conn *websocket.Conn, uuid_ uuid.UUID) {
	wsm.connectionsMtx.Lock()
	defer wsm.connectionsMtx.Unlock()

	wsm.connSubscriptions[conn] = uuid_

	connMap, ok := wsm.subscriptionConns[uuid_]
	if !ok {
		connMap = map[*websocket.Conn]struct{}{}
	}

	connMap[conn] = struct{}{}
	wsm.subscriptionConns[uuid_] = connMap

	log.Println("Subscribe", uuid_, len(connMap))
}

func (wsm *WSManager) unsubscribe(conn *websocket.Conn) error {
	wsm.connectionsMtx.Lock()
	defer wsm.connectionsMtx.Unlock()

	delete(wsm.connections, conn)

	uuid_, ok := wsm.connSubscriptions[conn]
	if !ok {
		// This isn't an error because the client could not subscribe over the connection
		return nil
	}

	delete(wsm.connSubscriptions, conn)

	connMap, ok := wsm.subscriptionConns[uuid_]
	// If there is no connection map, that just means they weren't watching a job
	// e.g. on home page.
	if !ok {
		return nil
	}

	delete(connMap, conn)
	if len(connMap) == 0 {
		delete(wsm.subscriptionConns, uuid_)
	}

	return nil
}

type Message map[string]interface{}

func (wsm *WSManager) Broadcast(uuid_ uuid.UUID, msg Message) error {
	wsm.connectionsMtx.RLock()
	defer wsm.connectionsMtx.RUnlock()

	connsMap, ok := wsm.subscriptionConns[uuid_]
	if !ok {
		return errors.Errorf("connection map not found for %v", uuid_)
	}

	for conn, _ := range connsMap {
		wsm.send(conn, msg)
	}

	return nil
}

func (wsm *WSManager) listen(conn *websocket.Conn) {
	defer func() {
		err := wsm.unsubscribe(conn)
		if err != nil {
			log.Println("unsubscribe error", errors.ErrorStack(err))
		}

		log.Println("Remove connection", wsm.NumConns())
	}()

	for {
		var v map[string]string
		err := wsjson.Read(context.Background(), conn, &v)
		if err != nil {
			// TODO capture whether this error was a page close or actual ws error
			// Client closed
			break
		}

		for cmd, arg := range v {
			switch cmd {
			case "subscribe":
				uuidParsed, err := uuid.Parse(arg)
				if err != nil {
					log.Println("WEBSOCKET PARSE ERROR", err)
					break
				}

				wsm.subscribe(conn, uuidParsed)
				wsm.send(conn, Message{"subscribed": arg})
			}
		}
	}
}

func (wsm *WSManager) send(conn *websocket.Conn, msg Message) error {
	log.Println("Send", msg)

	if err := wsjson.Write(context.Background(), conn, msg); err != nil {
		return errors.Annotate(err, "WSManager.send")
	}
	return nil
}

func (wsm *WSManager) NumConns() int {
	return len(wsm.connections)
}
