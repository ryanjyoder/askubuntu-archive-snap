package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"

	_ "github.com/mattn/go-sqlite3"
	askubuntu "github.com/ryanjyoder/askubuntu/pkg"
)

func main() {

	postXml, ok := os.LookupEnv("POSTS_XML")
	if !ok {
		log.Fatal("Must supply POSTS_XML")
	}

	indexFile, ok := os.LookupEnv("INDEX_DB")
	if !ok {
		log.Fatal("Must supply INDEX_DB")
	}

	if len(os.Args) < 2 {
		log.Fatal("Question Id must be provided as the first argument")
	}

	questionID, err := strconv.ParseInt(os.Args[1], 10, 64)
	if err != nil {
		log.Fatal(err)
	}

	store, err := askubuntu.NewXmlStoreFromFilenames(postXml, indexFile)
	if err != nil {
		log.Fatal("couldn't get data store:", err)
	}

	ctx := context.Background()
	question, err := store.GetQuestion(ctx, questionID)
	if err != nil {
		log.Fatal("couldn't find question:", err)
	}

	jsonbytes, _ := json.Marshal(question)
	fmt.Println(string(jsonbytes))

}

func ignore(...interface{}) {

}
