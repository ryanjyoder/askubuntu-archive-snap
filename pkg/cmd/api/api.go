package main

import (
	"fmt"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
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

	templatesDir, ok := os.LookupEnv("TEMPLATES_DIR")
	if !ok {
		log.Fatal("Must supply TEMPLATES_DIR")
	}

	listenPort, ok := os.LookupEnv("LISTEN_PORT")
	if !ok {
		listenPort = "8080"
	}
	if _, err := strconv.Atoi(listenPort); err != nil {
		log.Fatal("LISTEN_PORT must be an integer")
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
		log.Fatalln("Error opening store: ", err)
	}

	r := gin.New()
	r.LoadHTMLFiles(filepath.Join(templatesDir, "questions.html"), filepath.Join(templatesDir, "search.html"))

	wireRoutes(&r.RouterGroup, store)
	r.Run(":" + listenPort)
}
func wireRoutes(r *gin.RouterGroup, store *askubuntu.XmlStore) error {
	r.GET("/", func(c *gin.Context) {
		randID := int64(1 + rand.Int()%1214003)
		_, err := store.GetQuestion(c, randID)
		attemptsLeft := 15
		for err != nil {
			_, err = store.GetQuestion(c, randID)
			attemptsLeft--
		}
		randomURL := fmt.Sprintf("/questions/%d", randID)
		c.Redirect(http.StatusTemporaryRedirect, randomURL)
	})

	api := r.Group("/api")
	getQuestionJson := func(c *gin.Context) {
		idStr := c.Param("id")
		questionID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, err.Error())
			return
		}
		question, err := store.GetQuestion(c, questionID)
		if err != nil {
			c.JSON(http.StatusBadRequest, "GetQuestionFailed: "+err.Error())
			return
		}

		c.JSON(http.StatusOK, question)
	}

	api.GET("/questions/:id/:q", getQuestionJson)
	api.GET("/questions/:id", getQuestionJson)

	r.GET("/questions/:id", func(c *gin.Context) {
		idStr := c.Param("id")
		questionID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, err.Error())
			return
		}
		question, err := store.GetQuestion(c, questionID)
		if err != nil {
			c.JSON(http.StatusBadRequest, "GetQuestion failed: "+err.Error())
			return
		}

		args := map[string]interface{}{}
		args["Title"] = question.Title
		args["Body"] = template.HTML(question.Body)
		answers := make([]interface{}, len(question.Answers))
		for i := range question.Answers {
			answer := map[string]interface{}{}
			answer["Body"] = template.HTML(question.Answers[i].Body)
			answers[i] = answer
		}
		args["Answers"] = answers
		c.HTML(http.StatusOK, "questions.html", args)

	})

	r.GET("/search", func(c *gin.Context) {
		queryParam, ok := c.GetQuery("q")
		if !ok {
			c.JSON(http.StatusBadRequest, "you must provide a query string")
			return
		}
		terms := strings.Split(queryParam, " ")

		results, err := store.Search(c, terms)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		c.HTML(http.StatusOK, "search.html", results)
	})

	return nil

}

func ignore(...interface{}) {

}
