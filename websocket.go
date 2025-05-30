package main

import (
	"database/sql"
	"io"
	"os"
	"strings"
	"time"

	"github.com/google/logger"
	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 600 * time.Second

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
		c.hub.unregister <- c
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
	c.conn.SetReadLimit(4096)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func newHub() *Hub {
	return &Hub{
		broadcast:  make(chan statusMessage, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		refresh:    make(chan bool, 256),
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			logger.Infof("registered client %#v", client)
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				logger.Infof("unregistered client %#v", client)
			}
		case message := <-h.broadcast:
			// drain the queue
			r := message.RefreshNeeded
			l := message.LogMessages
			ql := len(h.broadcast)
			for i := 0; i < ql; i++ {
				nm := <-h.broadcast
				if nm.RefreshNeeded {
					r = true
				}
				if len(nm.LogMessages) > 0 {
					l = nm.LogMessages
				}
			}
			// send only the most relevant message
			message.RefreshNeeded = r
			message.LogMessages = l
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

// feedSockets collects and collates messages to be sent to websocket clients.
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
			logger.Info("received request to refresh statusz pages")
			// Coalesce multiple refresh events to one.
			time.Sleep(500 * time.Millisecond)
			qd := len(h.refresh)
			for range qd {
				<-h.refresh
			}
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
			if len(wsu.LogMessages) > 0 {
				h.broadcast <- wsu
			}
		}
	}
}

// processLogRows processes log files from SQL rows containing job IDs and log file locations.
// It reads each row to get the job ID and log file path, tails the log file to extract the last line,
// and stores the message in a map indexed by the job ID. If any error occurs during this process,
// it logs the error and continues processing the remaining rows. If there are no errors, it returns
// the map of log messages indexed by job ID; otherwise, it returns an error if one occurred.
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

// tailLog reads the last line from a given file path. It opens the file, moves to the end, and then reads backwards until it finds a newline character or reaches the buffer size limit.
// The function takes a single argument filePath of type string, which represents the path to the log file.
// It returns a string containing the last line of the file and an error if any occurred during the process.
func tailLog(filePath string) (string, error) {
	file, err := os.OpenFile(filePath, os.O_RDONLY, 0644)
	if err != nil {
		return "", err
	}
	defer file.Close()

	fileStat, err := os.Stat(filePath)
	if err != nil {
		return "", err
	}

	bufferSize := int64(1024) // Adjust buffer size as needed
	var lastLine []byte
	for i := int64(2); i <= bufferSize && i < fileStat.Size(); i++ {
		// Move the cursor back by `i` bytes from the end of the file.
		_, err := file.Seek(-i, io.SeekEnd)
		if err != nil {
			return "", err
		}
		char := make([]byte, 1)
		n, err := file.Read(char)
		if n == 0 || err != nil {
			// If we've reached the beginning of the file or there's an error, stop reading.
			break
		}
		if char[0] == '\n' || char[0] == '\r' {
			// Found a newline character, read the bytes to get the last line.
			lastLine = make([]byte, i-1)
			_, err := file.Seek(-i+1, io.SeekEnd)
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

	// If no newline was found (e.g., the entire file is a single line), read from the end backwards to the buffer size.
	if len(lastLine) == 0 {
		lastLine = make([]byte, min(fileStat.Size(), bufferSize))
		file.Seek(-int64(len(lastLine)), io.SeekEnd)
		_, err := file.Read(lastLine)
		if err != nil {
			return "", err
		}
	}

	return strings.TrimSpace(string(lastLine)), nil
}
