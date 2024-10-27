package main

import (
	"database/sql"
	"os"
	"time"

	"github.com/google/logger"
	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 5 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 30 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10
)

type statusMessage struct {
	LogMessages   map[int]string `json:"LogMessages"`
	RefreshNeeded bool           `json:"RefreshNeeded"`
}

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	hub *Hub

	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan statusMessage
}

type Hub struct {
	// Registered clients.
	clients map[*Client]bool

	// Inbound messages from the clients.
	broadcast chan statusMessage

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client

	// Notify page to refresh
	refresh chan bool
}

// writePump pumps messages from the hub to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteJSON(message); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(4096)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Errorf("error: %v", err)
			}
			break
		}
	}
}

func newHub() *Hub {
	return &Hub{
		broadcast:  make(chan statusMessage, 5),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		refresh:    make(chan bool),
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
		case message := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}

func (h *Hub) feedSockets() {
	pstmt, err := db.Prepare(`
	SELECT id, logfile
	FROM log_files
	WHERE id IN (SELECT id FROM active_jobs)
	`)
	if err != nil {
		logger.Errorf("failed to prepare statement: %v", err)
		logger.Warning("continuing without log tailing")
		return
	}

	rt := time.NewTicker(1 * time.Second)

	for {
		wsu := statusMessage{
			RefreshNeeded: false,
			LogMessages:   make(map[int]string),
		}
		select {
		case wsu.RefreshNeeded = <-h.refresh:
			h.broadcast <- wsu
			wsu.RefreshNeeded = false
		case <-rt.C:
			lf, err := pstmt.Query()
			if err != nil && err != sql.ErrNoRows {
				logger.Errorf("failed to query for log files: %v", err)
				continue
			}
			wsu.LogMessages, err = processLogRows(lf)
			if err != nil {
				logger.Errorf("could not get log tails: %v", err)
			}
			lf.Close()
			h.broadcast <- wsu
		}
	}
}

func processLogRows(rows *sql.Rows) (map[int]string, error) {
	var logMessages = make(map[int]string)
	row := struct {
		id   int
		file string
	}{}
	var err error
	for rows.Next() {
		err = rows.Scan(&row.id, &row.file)
		if err != nil && err != sql.ErrNoRows {
			logger.Errorf("failed to scan for log files: %v", err)
			continue
		}

		m, err := tailLog(row.file)
		if err != nil {
			logger.Errorf("failed to tail log: %v", err)
			continue
		}
		logMessages[row.id] = m
	}
	if err != nil && err != sql.ErrNoRows {
		logger.Errorf("failed processing log rows: %v", err)
		return nil, err
	}
	return logMessages, nil
}

func tailLog(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Move the cursor to the end of the file and start reading backwards.
	fileInfo, err := file.Stat()
	if err != nil {
		return "", err
	}
	fileSize := fileInfo.Size()
	bufferSize := int64(4096) // Adjust buffer size as needed
	var lastLine []byte
	for i := int64(1); i <= bufferSize; i++ {
		// Move the cursor back by `i` bytes from the end of the file.
		_, err := file.Seek(fileSize-i, os.SEEK_END)
		if err != nil {
			return "", err
		}
		char := make([]byte, 1)
		n, err := file.Read(char)
		if n == 0 || err != nil {
			// If we've reached the beginning of the file or there's an error, stop reading.
			break
		}
		if char[0] == '\n' {
			// Found a newline character, read the preceding bytes to get the last line.
			lastLine = make([]byte, i-1)
			_, err := file.Seek(fileSize-i+1, os.SEEK_END)
			if err != nil {
				return "", err
			}
			n, err := file.Read(lastLine)
			if n == 0 || err != nil {
				return "", err
			}
			break
		}
	}

	// If no newline was found (e.g., the entire file is a single line), read from the beginning to the buffer size.
	if len(lastLine) == 0 {
		lastLine = make([]byte, bufferSize)
		_, err := file.Read(lastLine)
		if err != nil {
			return "", err
		}
	}

	return string(lastLine), nil
}
