package askubuntu

import (
	"context"
	"database/sql"
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/ryanjyoder/sofp"
	"github.com/vanng822/go-solr/solr"
)

type Store interface {
	GetQuestion(ctx context.Context, questionID int64) (*sofp.Question, error)
	Search(ctx context.Context, searchTerms []string) ([]SearchResult, error)
}

type XmlStore struct {
	xmlFile    io.ReadSeeker
	indexDB    SqlDB
	query      *sql.Stmt
	solrI      *solr.SolrInterface
	queryMutex sync.Mutex
}
type SqlDB interface {
	Prepare(query string) (*sql.Stmt, error)
}

type SearchResult struct {
	SearchScore int
	Title       string
	Summary     string
	QuestionID  string
	PostID      string
	IsQuestion  bool
}

type StoreConfigs struct {
	XmlFilename    string
	DBFilename     string
	SolrURL        string
	SolrUser       string
	SolrPassword   string
	SolrCollection string
}

func NewXmlStoreFromConfigs(conf StoreConfigs) (*XmlStore, error) {
	database, err := sql.Open("sqlite3", conf.DBFilename)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(conf.XmlFilename)
	if err != nil {
		return nil, err
	}

	u, err := url.Parse(conf.SolrURL)
	u.User = url.UserPassword(conf.SolrUser, conf.SolrPassword)
	si, err := solr.NewSolrInterface(u.String(), conf.SolrCollection)
	if err != nil {
		return nil, err
	}

	return NewXmlStore(file, database, si)

}

func NewXmlStore(xmlFile io.ReadSeeker, db SqlDB, si *solr.SolrInterface) (*XmlStore, error) {
	s := &XmlStore{
		xmlFile: xmlFile,
		indexDB: db,
		solrI:   si,
	}
	stmt, err := db.Prepare(`
		SELECT 
			start, length 
		FROM posts 
		WHERE question_id = (SELECT question_id FROM posts WHERE id = ? LIMIT 1) `)
	if err != nil {
		return nil, err
	}
	s.query = stmt

	return s, nil
}

func (s *XmlStore) GetQuestion(ctx context.Context, questionID int64) (*sofp.Question, error) {
	s.queryMutex.Lock()
	defer s.queryMutex.Unlock()
	rows, err := s.query.QueryContext(ctx, questionID)
	if err != nil {
		return nil, err
	}
	var start int64
	var length int64
	question := &sofp.Question{}
	answers := []*sofp.Answer{}
	for rows.Next() {
		rows.Scan(&start, &length)
		_, err = s.xmlFile.Seek(start, 0)
		if err != nil {
			return nil, err
		}

		b := make([]byte, length)
		n, err := s.xmlFile.Read(b)
		if err != nil {
			return nil, err
		}
		if n != len(b) {
			return nil, fmt.Errorf("could not read entire post")
		}
		row := sofp.Row{}
		xml.Unmarshal(b, &row)
		if row.PostTypeID == "1" {
			q, err := row.GetQuestion()
			if err != nil {
				return nil, err
			}
			question = q
		}
		if row.PostTypeID == "2" {
			a, err := row.GetAnswer()
			if err != nil {
				return nil, err
			}
			answers = append(answers, a)
		}

	}
	sort.SliceStable(answers, func(i, j int) bool {
		// Highest scores first >
		return answers[i].Score > answers[j].Score
	})

	if question.ID == 0 {
		return nil, fmt.Errorf("quetion not found")
	}
	question.Answers = answers
	return question, nil
}

func (s *XmlStore) Search(ctx context.Context, searchTerms []string) ([]SearchResult, error) {
	qConditions := []string{}
	invalidChar := regexp.MustCompile(`"':`)

	for _, term := range searchTerms {
		t := invalidChar.ReplaceAllString(term, "")
		cond := fmt.Sprintf(`(Title:"%s" OR Body:"%s")`, t, t)
		qConditions = append(qConditions, cond)
	}

	phrase := invalidChar.ReplaceAllString(strings.Join(searchTerms, " "), "")
	boostQStr := fmt.Sprintf(`Title:"%s"~50 OR Body:"%s"~110`, phrase, phrase)

	//qStr := fmt.Sprintf(`Title:"%s"~100 OR Body:"%s"~100`, phrase, phrase)
	qStr := strings.Join(qConditions, " OR ")
	query := solr.NewQuery()
	query.Q(qStr)
	query.BoostQuery(boostQStr)
	sq := s.solrI.Search(query)
	r, err := sq.Result(nil)
	if err != nil {
		return nil, fmt.Errorf("solr Results error: %v", err)
	}
	results := make([]SearchResult, len(r.Results.Docs))

	for i, doc := range r.Results.Docs {
		results[i].PostID = interfaceToString(doc["Id"])
		results[i].QuestionID = interfaceToString(doc["QuestionId"])
		results[i].Title = interfaceToString(doc["Title"])
		results[i].Summary = interfaceToString(doc["Summary"])
	}

	return results, nil
}

func interfaceToString(i interface{}) string {
	if i == nil {
		return ""
	}
	if s, ok := i.(string); ok {
		return s
	}
	return ""
}
