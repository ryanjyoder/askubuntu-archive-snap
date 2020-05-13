package main

import (
	"bufio"
	"context"
	"encoding/xml"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"

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
	docs := make(chan string)

	go func() {
		defer close(docs)

		scanner := bufio.NewScanner(file)
		content, errs := getContent(scanner)
		wg := sync.WaitGroup{}
		maxWorkers := runtime.GOMAXPROCS(0) + 2
		sem := semaphore.NewWeighted(int64(maxWorkers))
		ctx := context.TODO()
		fmt.Fprintf(os.Stderr, "parsing with %d workds\n", maxWorkers)

		for line := range content {
			if err := sem.Acquire(ctx, 1); err != nil {
				log.Printf("Failed to acquire semaphore: %v", err)
				break
			}
			wg.Add(1)

			go func(l string) {
				defer wg.Done()
				defer sem.Release(1)

				doc, err := formatDocument(l)
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					return
				}
				docs <- doc
			}(line)
		}
		wg.Wait()
		if err := <-errs; err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}()

	fmt.Println(`<?xml version="1.0" ?>`)
	fmt.Println("<add>")
	for doc := range docs {
		fmt.Println(doc)
	}
	fmt.Println("</add>")

}

type Doc struct {
	XMLName xml.Name `xml:"doc"`
	Fields  []Field  `xml:"field"`
}
type Field struct {
	Name  string `xml:"name,attr"`
	Value string `xml:",chardata"`
}

func formatDocument(line string) (string, error) {
	row := &sofp.Row{}
	if err := xml.Unmarshal([]byte(line), row); err != nil {
		return "", err
	}
	if row.ID == nil {
		return "", fmt.Errorf("nil PostId")
	}
	questionID := pointerToInt(row.ParentID)
	if questionID == 0 {
		questionID = *row.ID
	}

	body, err := html2text.FromString(row.Body, html2text.Options{})
	if err != nil {
		return "", err
	}

	summary := summarize(body)

	doc := Doc{Fields: []Field{{
		Name:  "Id",
		Value: fmt.Sprintf("%d", *row.ID),
	}, {
		Name:  "QuestionId",
		Value: fmt.Sprintf("%d", questionID),
	}, {
		Name:  "Title",
		Value: row.Title,
	}, {
		Name:  "Body",
		Value: body,
	}, {
		Name:  "Summary",
		Value: summary,
	}}}

	xmlBytes, _ := xml.Marshal(doc)
	return string(xmlBytes), nil

}

func summarize(s string) string {
	maxLen := 200
	if len(s) < maxLen {
		return s
	}
	words := strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == '\n'
	})

	curLen := 0
	lastWordInSum := 0
	for i := range words {
		additionalLen := 1 + len(words[i])
		if curLen+additionalLen > maxLen {
			break
		}
		lastWordInSum = i
		curLen = curLen + additionalLen
	}
	return strings.Join(words[:lastWordInSum], " ")
}

func pointerToString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func pointerToInt(i *int) int {
	if i == nil {
		return 0
	}
	return *i
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
		defer close(lines)
		defer close(errs)
		for scanner.Scan() {
			line := scanner.Text()
			lines <- line
		}
		if err := scanner.Err(); err != nil {
			errs <- err
		}
	}()

	return lines, errs
}
