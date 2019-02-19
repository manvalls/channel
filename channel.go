package channel

import (
	"context"
	"errors"
	"sync"

	"github.com/manvalls/wit"
)

// ErrNoContextOrSocket is returned when there are missing parameters in the Join function
var ErrNoContextOrSocket = errors.New("You need to provide a valid Context and Socket channel")

// Channel holds the channel information
type Channel struct {
	mutex   *sync.Mutex
	clients map[string]map[*struct{}]JoinOptions
}

// NewChannel builds a new channel
func NewChannel() Channel {
	return Channel{&sync.Mutex{}, map[string]map[*struct{}]JoinOptions{}}
}

// JoinOptions includes the list of options that Join accepts
type JoinOptions interface {
	CmdCh() chan<- wit.Command
	ClientID() string
	context.Context
}

// Join adds a client to the channel, waits until the context is done and removes it
func (c Channel) Join(options JoinOptions) error {
	key := &struct{}{}

	c.mutex.Lock()

	cmap, ok := c.clients[options.ClientID()]
	if !ok {
		cmap = map[*struct{}]JoinOptions{}
		c.clients[options.ClientID()] = cmap
	}

	cmap[key] = options

	c.mutex.Unlock()

	<-options.Done()

	c.mutex.Lock()

	delete(cmap, key)
	if len(cmap) == 0 {
		delete(c.clients, options.ClientID())
	}

	c.mutex.Unlock()

	return nil
}

// Send sends a message to a list of clients
func (c Channel) Send(cmd wit.Command, clientIDs ...string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for _, clientID := range clientIDs {
		if cmap, ok := c.clients[clientID]; ok {
			for _, op := range cmap {
				select {
				case <-op.Done():
				case op.CmdCh() <- cmd:
				}
			}
		}
	}
}

// Broadcast sends a message to the whole channel except for the provided IDs
func (c Channel) Broadcast(cmd wit.Command, blacklistClientIDs ...string) {
	blacklisted := make(map[string]bool)
	for _, clientID := range blacklistClientIDs {
		blacklisted[clientID] = true
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	for clientID, cmap := range c.clients {
		if !blacklisted[clientID] {
			for _, op := range cmap {
				select {
				case <-op.Done():
				case op.CmdCh() <- cmd:
				}
			}
		}
	}
}
