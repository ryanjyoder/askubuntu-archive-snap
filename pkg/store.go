package askubuntu

import (
	"context"
	"database/sql"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"sync"

	"github.com/ryanjyoder/sofp"
)

type XmlStore struct {
	xmlFile    io.ReadSeeker
	indexDB    SqlDB
	query      *sql.Stmt
	queryMutex sync.Mutex
}
type SqlDB interface {
	Prepare(query string) (*sql.Stmt, error)
}

func NewXmlStoreFromFilenames(xmlFilename string, dbFilename string) (*XmlStore, error) {
	database, err := sql.Open("sqlite3", dbFilename)
	if err != nil {
		log.Fatal(err)
	}

	file, err := os.Open(xmlFilename)
	if err != nil {
		log.Fatal(err)
	}

	return NewXmlStore(file, database)

}

func NewXmlStore(xmlFile io.ReadSeeker, db SqlDB) (*XmlStore, error) {
	s := &XmlStore{
		xmlFile: xmlFile,
		indexDB: db,
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
