// Package events - Pub-Sub in go with event caching
package events

import (
	"sync"

	cmn "github.com/tendermint/classic/libs/common"
)

// All implementors must be amino-encodable.
type Event interface {
	AssertEvent()
}

// Eventable is the interface reactors and other modules must export to become
// eventable.
type Eventable interface {
	SetEventSwitch(evsw EventSwitch)
}

// Fireable is the interface that wraps the FireEvent method.
//
// FireEvent fires an event with the given name and data.
type Fireable interface {
	FireEvent(ev Event)
}

// EventSwitch is the interface for synchronous pubsub, where listeners
// subscribe to certain events and, when an event is fired (see Fireable),
// notified via a callback function.
// All listeners are expected to perform work quickly and not block processing
// of the main event emitter.
type EventSwitch interface {
	cmn.Service
	Fireable

	// Multiple callbacks can be registered for each listener.
	AddListener(listenerID string, cb EventCallback)
	// Removes all callbacks for listener.
	RemoveListener(listenerID string)
}

type EventCallback func(event Event)

type listenCell struct {
	listenerID string
	cb         EventCallback
}

// This simple implementation is optimized for few listeners.
// This is faster for few listeners, especially for FireEvent.
type eventSwitch struct {
	cmn.BaseService

	mtx       sync.RWMutex
	listeners []listenCell
}

func NilEventSwitch() EventSwitch {
	return (*eventSwitch)(nil)
}

func NewEventSwitch() EventSwitch {
	evsw := &eventSwitch{
		listeners: make([]listenCell, 0, 10),
	}
	evsw.BaseService = *cmn.NewBaseService(nil, "EventSwitch", evsw)
	return evsw
}

func (evsw *eventSwitch) OnStart() error {
	return nil
}

func (evsw *eventSwitch) OnStop() {}

func (evsw *eventSwitch) AddListener(listenerID string, cb EventCallback) {
	evsw.mtx.Lock()
	evsw.listeners = append(evsw.listeners, listenCell{listenerID, cb})
	evsw.mtx.Unlock()
}

func (evsw *eventSwitch) RemoveListener(listenerID string) {
	evsw.mtx.Lock()
	newlisteners := make([]listenCell, 0, len(evsw.listeners))
	for _, cell := range evsw.listeners {
		if cell.listenerID != listenerID {
			newlisteners = append(newlisteners, cell)
		}
	}
	evsw.listeners = newlisteners
	evsw.mtx.Unlock()
}

// FireEvent on a nil switch is a noop, but no other operations are allowed for
// safety.
func (evsw *eventSwitch) FireEvent(event Event) {
	if evsw == nil {
		return
	}
	evsw.mtx.RLock()
	listeners := make([]listenCell, len(evsw.listeners))
	copy(listeners, evsw.listeners)
	evsw.mtx.RUnlock()

	for _, cell := range listeners {
		cell.cb(event)
	}
}
