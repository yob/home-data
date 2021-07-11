package memorystate

import (
	"sync"
)

type State struct {
	data *sync.Map
}

type readOnlyState struct {
	writeableState *State
}

type StateReader interface {
	Read(string) (string, bool)
}

func New() *State {
	return &State{
		data: &sync.Map{},
	}
}

func (state *State) Read(key string) (string, bool) {
	if value, ok := state.data.Load(key); ok {
		return value.(string), true
	}
	return "", false
}

func (state *State) ReadOnly() *readOnlyState {
	return &readOnlyState{
		writeableState: state,
	}
}

func (state *State) Store(key string, value string) {
	state.data.Store(key, value)
}

func (state *readOnlyState) Read(key string) (string, bool) {
	return state.writeableState.Read(key)
}
