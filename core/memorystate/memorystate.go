package memorystate

import (
	"strconv"
	"sync"
	"time"

	"github.com/yob/home-data/core/homestate"
)

type State struct {
	data *sync.Map
}

type readOnlyState struct {
	writeableState *State
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

func (state *State) ReadFloat64(key string) (float64, bool) {
	if value, ok := state.data.Load(key); ok {
		value64, err := strconv.ParseFloat(value.(string), 8)
		if err != nil {
			return 0, false
		}
		return value64, true
	}
	return 0, false
}

func (state *State) ReadTime(key string) (time.Time, bool) {
	if strTime, ok := state.data.Load(key); ok {
		t, err := time.Parse(time.RFC3339, strTime.(string))
		if err != nil {
			return time.Now(), false
		}
		return t, true
	}
	return time.Now(), false
}

func (state *State) ReadOnly() homestate.StateReader {
	return &readOnlyState{
		writeableState: state,
	}
}

func (state *State) Store(key string, value string) error {
	state.data.Store(key, value)
	return nil
}

func (state *readOnlyState) Read(key string) (string, bool) {
	return state.writeableState.Read(key)
}

func (state *readOnlyState) ReadFloat64(key string) (float64, bool) {
	return state.writeableState.ReadFloat64(key)
}

func (state *readOnlyState) ReadTime(key string) (time.Time, bool) {
	return state.writeableState.ReadTime(key)
}
