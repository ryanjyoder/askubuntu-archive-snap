package main

import (
	"log"
	"net/http"
	"os"
	"strconv"

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

	wireRoutes(&r.RouterGroup, store)
	r.Run(":" + listenPort)
}
func wireRoutes(r *gin.RouterGroup, store *askubuntu.XmlStore) error {

	r.GET("/questions/:id", func(c *gin.Context) {
		idStr := c.Param("id")
		questionID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, err.Error())
			return
		}
		question, err := store.GetQuestion(c, questionID)
		if err != nil {
			c.JSON(http.StatusBadRequest, err.Error())
			return
		}

		c.JSON(http.StatusOK, question)
	})

	return nil

}

func ignore(...interface{}) {

}
