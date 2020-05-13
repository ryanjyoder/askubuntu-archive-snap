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

const (
	QUESTIONS_CMD = "questions"
	SEARCH_CMD    = "search"
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

	store, err := askubuntu.NewXmlStoreFromConfigs(askubuntu.StoreConfigs{
		XmlFilename:    postXml,
		DBFilename:     indexFile,
		SolrURL:        "http://localhost:8983/solr",
		SolrUser:       "guest",
		SolrPassword:   "SolrRocks",
		SolrCollection: "askubuntu",
	})
	if err != nil {
		fmt.Errorf("couldn't get data store: %v", err)
	}

	if len(os.Args) < 2 {
		log.Fatal("First argument must be either, 'questions' or 'search'")
	}

	cmdArgs := os.Args[2:]
	switch os.Args[1] {
	case QUESTIONS_CMD:
		err := doQuestions(store, cmdArgs)
		if err != nil {
			log.Fatalln(err)
		}
	case SEARCH_CMD:
		err := doSearch(store, cmdArgs)
		if err != nil {
			log.Fatalln(err)
		}
	default:
		log.Fatalln("Unknown command:", os.Args[1])
	}
}

func doQuestions(store askubuntu.Store, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("questions command requires a question id as the second argument")
	}

	questionID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return err
	}

	ctx := context.Background()
	question, err := store.GetQuestion(ctx, questionID)
	if err != nil {
		fmt.Errorf("couldn't find question: %v", err)
	}

	jsonbytes, _ := json.Marshal(question)
	fmt.Println(string(jsonbytes))
	return nil
}

func doSearch(store askubuntu.Store, args []string) error {
	ctx := context.Background()
	results, err := store.Search(ctx, args)
	if err != nil {
		return fmt.Errorf("couldn't find question: %v", err)
	}

	jsonbytes, _ := json.Marshal(results)
	fmt.Println(string(jsonbytes))
	return nil
}

func ignore(...interface{}) {

}
