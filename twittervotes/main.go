package main

import (
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

func main() {}
