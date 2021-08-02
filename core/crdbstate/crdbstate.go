package crdbstate

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"strconv"
	"time"

	conf "github.com/yob/home-data/core/config"
	"github.com/yob/home-data/core/homestate"
)

type State struct {
	db *sql.DB
}

type readOnlyState struct {
	writeableState *State
}

func New(config *conf.ConfigSection) (*State, error) {
	connectString, err := config.GetString("connect")
	if err != nil {
		return nil, fmt.Errorf("crdbstate: connect not set in config - %v", err)
	}
	db, err := sql.Open("postgres", connectString)

	if err != nil {
		return nil, fmt.Errorf("error connecting to the database: %v", err)
	}

	// Create the "values" table.
	if _, err := db.Exec(
		"CREATE TABLE IF NOT EXISTS values (id INT PRIMARY KEY DEFAULT unique_rowid(), key TEXT NOT NULL UNIQUE, value TEXT NOT NULL)"); err != nil {
		return nil, fmt.Errorf("error creating db table: %v", err)
	}

	return &State{
		db: db,
	}, nil
}

func (state *State) Read(key string) (string, bool) {
	var result string
	err := state.db.QueryRow("SELECT value FROM values WHERE key = $1", key).Scan(&result)

	if err == sql.ErrNoRows {
		return "", false
	} else if err != nil {
		fmt.Printf("crdb select error: %v\n", err)
		return "", false
	}
	return result, true
}

func (state *State) ReadFloat64(key string) (float64, bool) {
	if value, ok := state.Read(key); ok {
		value64, err := strconv.ParseFloat(value, 8)
		if err != nil {
			return 0, false
		}
		return value64, true
	}
	return 0, false
}

func (state *State) ReadTime(key string) (time.Time, bool) {
	if strTime, ok := state.Read(key); ok {
		t, err := time.Parse(time.RFC3339, strTime)
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
	if _, err := state.db.Exec(
		"INSERT INTO values (key, value) VALUES ($1, $2) ON CONFLICT (key) DO UPDATE SET value = excluded.value", key, value); err != nil {
		fmt.Printf("crdb.Store: %v\n", err)
		return err
	}
	return nil
}

func (state *State) StoreMulti(updates map[string]string) error {
	tx, err := state.db.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	_, err = tx.Exec("SET statement_timeout = 3000")
	if err != nil {
		return err
	}

	for key, value := range updates {
		if _, err := tx.Exec(
			"INSERT INTO values (key, value) VALUES ($1, $2) ON CONFLICT (key) DO UPDATE SET value = excluded.value", key, value); err != nil {
			fmt.Printf("crdb.Store: %v\n", err)
			return err
		}
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

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
