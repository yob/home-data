package homestate

import (
	"time"
)

type State interface {
	Read(string) (string, bool)
	ReadFloat64(string) (float64, bool)
	ReadTime(string) (time.Time, bool)
	Store(string, string) error
	StoreMulti(map[string]string) error
	Remove(string) error
	ReadOnly() StateReader
}

type StateReader interface {
	Read(string) (string, bool)
	ReadFloat64(string) (float64, bool)
	ReadTime(string) (time.Time, bool)
}
