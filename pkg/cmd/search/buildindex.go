package main

import (
	"bufio"
	"context"
	"encoding/xml"
	"fmt"
	"log"
	"os"
	"runtime"
	"sync"

	"github.com/jdkato/prose/tokenize"
	"github.com/ryanjyoder/sofp"
	"golang.org/x/sync/semaphore"
	"jaytaylor.com/html2text"
)

func main() {

	file := os.Stdin
	var err error
	if len(os.Args) > 1 {
		xmlFilename := os.Args[1]
		file, err = os.Open(xmlFilename)
		if err != nil {
			log.Fatal(err)
		}
	}
	words := make(chan string)

	go func() {
		defer close(words)

		scanner := bufio.NewScanner(file)
		content, errs := getContent(scanner)
		wg := sync.WaitGroup{}
		maxWorkers := runtime.GOMAXPROCS(0)
		sem := semaphore.NewWeighted(int64(maxWorkers))
		ctx := context.TODO()
		fmt.Fprintf(os.Stderr, "parsing with %d workds\n", maxWorkers)

		for line := range content {
			if err := sem.Acquire(ctx, 1); err != nil {
				log.Printf("Failed to acquire semaphore: %v", err)
				break
			}
			wg.Add(1)

			go func() {
				defer wg.Done()
				defer sem.Release(1)

				tokenizeText(line, words)
			}()
		}
		wg.Wait()
		if err := <-errs; err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}()

	for word := range words {
		fmt.Println(word)
	}

}

func tokenizeText(line string, out chan string) {
	text, _ := html2text.FromString(line)
	words := tokenize.TextToWords(text)
	uniqueWords := map[string]bool{}
	for _, word := range words {
		uniqueWords[word] = true
	}
	for word := range uniqueWords {
		out <- word
	}
}

type Scanner interface {
	Scan() bool
	Text() string
	Err() error
}

func getContent(scanner Scanner) (<-chan string, <-chan error) {
	lines := make(chan string)
	errs := make(chan error)

	go func() {
		for scanner.Scan() {
			defer close(lines)
			defer close(errs)

			line := scanner.Text()
			row := &sofp.Row{}
			if err := xml.Unmarshal([]byte(line), row); err != nil {
				log.Println("error parsing row:", err)
				continue
			}

			lines <- row.Title + " " + row.Body
		}
		if err := scanner.Err(); err != nil {
			errs <- err
		}
	}()

	return lines, errs
}
