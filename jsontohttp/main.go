package main

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/tidwall/gjson"
	"github.com/urfave/cli/v2"
)

const (
	workers = 5
)

func main() {
	var targetUrl string

	app := &cli.App{
		Name:  "jsontohttp",
		Usage: "jsontohttp --target=\"http://example.com/foo\"",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "target",
				Usage:       "A HTTP URL to post each line of input to",
				Destination: &targetUrl,
			},
		},
		Action: func(c *cli.Context) error {
			if targetUrl == "" {
				return fmt.Errorf("target flag is requured")
			}
			return run(targetUrl)
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func run(targetUrl string) error {
	jsonObjects := make(chan string)
	wg := new(sync.WaitGroup)

	for w := 1; w <= workers; w++ {
		wg.Add(1)
		go worker(jsonObjects, wg, targetUrl)
	}

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		jsonBody := scanner.Text()
		if !gjson.Valid(jsonBody) {
			fmt.Fprintf(os.Stderr, "input is invalid JSON\n")
			continue
		}
		jsonObjects <- jsonBody
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	// Closing channel (waiting in worker goroutines won't continue any more)
	close(jsonObjects)

	wg.Wait()

	return nil
}

func worker(jsonObjects <-chan string, wg *sync.WaitGroup, targetUrl string) {
	defer wg.Done()

	for jsonObject := range jsonObjects {
		fmt.Printf("POST %s to %s\n", firstN(jsonObject, 25), targetUrl)
		resp, err := http.Post(targetUrl, "application/json", strings.NewReader(jsonObject))
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode != 200 {
			fmt.Fprintf(os.Stderr, "ERROR: non-200 response (%d)\n", resp.StatusCode)
			continue
		}
	}
}

func firstN(s string, n int) string {
	i := 0
	for j := range s {
		if i == n {
			return s[:j] + "..."
		}
		i++
	}
	return s
}
