package babyapi

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
)

type broadcastChannel[T any] struct {
	listeners []chan T
	lock      sync.RWMutex
}

func (bc *broadcastChannel[T]) GetListener() chan T {
	bc.lock.Lock()
	defer bc.lock.Unlock()
	newChan := make(chan T)
	bc.listeners = append(bc.listeners, newChan)
	return newChan
}

func (bc *broadcastChannel[T]) RemoveListener(removeChan chan T) {
	bc.lock.Lock()
	defer bc.lock.Unlock()
	for i, listener := range bc.listeners {
		if listener == removeChan {
			bc.listeners[i] = bc.listeners[len(bc.listeners)-1]
			bc.listeners = bc.listeners[:len(bc.listeners)-1]
			close(listener)
			return
		}
	}
}

func (bc *broadcastChannel[T]) SendToAll(input T) {
	bc.lock.RLock()
	defer bc.lock.RUnlock()
	for _, listener := range bc.listeners {
		listener <- input
	}
}

func (bc *broadcastChannel[T]) runInputChannel(inputChan chan T) {
	for input := range inputChan {
		bc.SendToAll(input)
	}
}

// GetInputChannel returns a channel acting as an input to the broadcast channel, closing the channel will stop the worker goroutine
func (bc *broadcastChannel[T]) GetInputChannel() chan T {
	newInputChan := make(chan T)
	go bc.runInputChannel(newInputChan)
	return newInputChan
}

// ServerSentEvent is a simple struct that represents an event used in HTTP event stream
type ServerSentEvent struct {
	Event string
	Data  string
}

// Write will write the ServerSentEvent to the HTTP response stream and flush. It removes all newlines
// in the event data
func (sse *ServerSentEvent) Write(w http.ResponseWriter) {
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", sse.Event, strings.ReplaceAll(sse.Data, "\n", ""))
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// AddServerSentEventHandler is a shortcut for HandleServerSentEvents that automatically creates and returns
// the events channel and adds a custom handler for GET requests matching the provided pattern
func (a *API[T]) AddServerSentEventHandler(pattern string) chan *ServerSentEvent {
	eventsBroadcastChannel := broadcastChannel[*ServerSentEvent]{}

	a.AddCustomRoute(http.MethodGet, pattern, a.HandleServerSentEvents(&eventsBroadcastChannel))

	return eventsBroadcastChannel.GetInputChannel()
}

// HandleServerSentEvents is a handler function that will listen on the provided channel and write events
// to the HTTP response
func (a *API[T]) HandleServerSentEvents(EventsBroadcastChannel *broadcastChannel[*ServerSentEvent]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		events := EventsBroadcastChannel.GetListener()
		defer EventsBroadcastChannel.RemoveListener(events)
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Content-Type", "text/event-stream")

		for {
			select {
			case e := <-events:
				e.Write(w)
			case <-r.Context().Done():
				return
			case <-a.Done():
				return
			}
		}
	}
}
