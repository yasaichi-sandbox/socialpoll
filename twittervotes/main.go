package main

import (
	"github.com/bitly/go-nsq"
	"gopkg.in/mgo.v2"
	"log"
)

var db *mgo.Session

func dialdb() error {
	var err error

	log.Println("MongoDBにダイヤル中: localhost")
	db, err = mgo.Dial("localhost")

	return err
}

func closedb() {
	if db == nil {
		return
	}

	db.Close()
	log.Println("データベース接続が閉じられました")
}

type poll struct {
	Options []string
}

func loadOptions() ([]string, error) {
	var options []string

	// nil stands for "no filtering"
	iter := db.DB("ballots").C("polls").Find(nil).Iter()

	var p poll
	for iter.Next(&p) {
		options = append(options, p.Options...)
	}
	iter.Close()

	return options, iter.Err()
}

func publishVotes(votes <-chan string) <-chan struct{} {
	stopchan := make(chan struct{}, 1)
	producer, _ := nsq.NewProducer("localhost:4150", nsq.NewConfig())

	go func() {
		for vote := range votes {
			producer.Publish("votes", []byte(vote))
		}

		log.Println("Publisher: 停止中です")
		producer.Stop()
		log.Println("Publisher: 停止しました")
		stopchan <- struct{}{}
	}()

	return stopchan
}

func main() {}
