package main

import (
	"github.com/bitly/go-nsq"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

const updateDuration = 1 * time.Second

func main() {
	err := counterMain()

	if err != nil {
		log.Fatal(err)
	}
}

func counterMain() error {
	log.Println("データベースに接続します...")
	db, err := mgo.Dial("localhost")
	if err != nil {
		return err
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
		return err
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
		return err
	}

	log.Println("NSQ上での投票を待機します...")
	ticker := time.NewTicker(updateDuration)
	defer ticker.Stop()

	pollData := db.DB("ballots").C("polls")
	update := func() {
		countsLock.Lock()
		defer countsLock.Unlock()

		if len(counts) == 0 {
			log.Println("新しい投票はありません。データベースの更新をスキップします")
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
	}

	termChan := make(chan os.Signal, 1)
	signal.Notify(termChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	for {
		select {
		case <-ticker.C:
			update()
		case <-termChan:
			consumer.Stop()
		case <-consumer.StopChan:
			return nil
		}
	}
}
