package channel

import (
	"context"
	"errors"
	"sync"

	"github.com/manvalls/wit"
)

// ErrNoContextOrSocket is returned when there are missing parameters in the Add function
var ErrNoContextOrSocket = errors.New("You need to provide a valid Context and Socket channel")

// Channel holds the channel information
type Channel struct {
	mutex   *sync.Mutex
	clients map[string]map[*struct{}]AddOptions
}

// NewChannel builds a new channel
func NewChannel() Channel {
	return Channel{&sync.Mutex{}, map[string]map[*struct{}]AddOptions{}}
}

// AddOptions includes the list of options that Add accepts
type AddOptions = struct {
	context.Context
	Socket   chan<- wit.Command
	ClientID string
}

// Join adds a client to the channel, waits until the context is done and removes it
func (c Channel) Join(options AddOptions) error {
	if options.Context == nil || options.Socket == nil {
		return ErrNoContextOrSocket
	}

	key := &struct{}{}

	c.mutex.Lock()

	cmap, ok := c.clients[options.ClientID]
	if !ok {
		cmap = map[*struct{}]AddOptions{}
		c.clients[options.ClientID] = cmap
	}

	cmap[key] = options

	c.mutex.Unlock()

	<-options.Context.Done()

	c.mutex.Lock()

	delete(cmap, key)
	if len(cmap) == 0 {
		delete(c.clients, options.ClientID)
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
				case op.Socket <- cmd:
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
				case op.Socket <- cmd:
				}
			}
		}
	}
}
