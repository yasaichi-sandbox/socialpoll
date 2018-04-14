package main

import (
	"flag"
	"fmt"
	"github.com/bitly/go-nsq"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"log"
	"os"
	"sync"
	"time"
)

var fatalErr error

func fatal(e error) {
	fmt.Println(e)
	flag.PrintDefaults()
	fatalErr = e
}

const updateDuration = 1 * time.Second

func main() {
	defer func() {
		if fatalErr != nil {
			os.Exit(1)
		}
	}()

	log.Println("データベースに接続します...")
	db, err := mgo.Dial("localhost")
	if err != nil {
		fatal(err)
		return
	}
	defer func() {
		log.Println("データベース接続を閉じます...")
		db.Close()
	}()

	var countsLock sync.Mutex
	var counts map[string]int

	log.Println("NSQに接続します...")
	consumer, err := nsq.NewConsumer("votes", "counter", nsq.NewConfig())
	if err != nil {
		fatal(err)
		return
	}

	// NOTE: `nsq.HandlerFunc()` does "Type conversion"
	consumer.AddHandler(nsq.HandlerFunc(func(m *nsq.Message) error {
		countsLock.Lock()
		defer countsLock.Unlock()

		// NOTE: counts can be `nil` when `updater` succeeds to update collection in the DB
		if counts == nil {
			counts = make(map[string]int)
		}

		vote := string(m.Body)
		counts[vote]++

		return nil
	}))

	if err := consumer.ConnectToNSQLookupd("localhost:4161"); err != nil {
		fatal(err)
		return
	}

	log.Println("NSQ上での投票を待機します...")
	var updater *time.Timer

	pollData := db.DB("ballots").C("polls")
	updater = time.AfterFunc(updateDuration, func() {
		countsLock.Lock()
		defer countsLock.Unlock()

		if len(counts) == 0 {
			log.Println("新しい投票はありません。データベースの更新をスキップします")
			updater.Reset(updateDuration)
			return
		}

		log.Println("データベースを更新します...")
		log.Println(counts)
		ok := true

		for option, count := range counts {
			// NOTE: `bson.M` stands for Map of BSON(Binary JSON)
			selector := bson.M{"options": bson.M{"$in": []string{option}}}
			update := bson.M{"$inc": bson.M{("results." + option): count}}

			if _, err := pollData.UpdateAll(selector, update); err != nil {
				log.Println("更新に失敗しました:", err)
				ok = false
				continue
			}

			counts[option] = 0
		}

		if ok {
			log.Println("データベースの更新が完了しました")
			counts = nil
		}

		updater.Reset(updateDuration)
	})
}
