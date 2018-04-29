package main

import (
	"flag"
	"github.com/stretchr/graceful"
	"gopkg.in/mgo.v2"
	"log"
	"net/http"
	"time"
)

func main() {
	addr := flag.String("addr", ":8080", "エンドポイントのアドレス")
	mongo := flag.String("mongo", "localhost", "MongoDBのアドレス")
	flag.Parse()

	log.Println("MongoDBに接続します", *mongo)
	db, err := mgo.Dial(*mongo)
	if err != nil {
		log.Fatalln("MongoDBへの接続に失敗しました:", err)
	}
	defer db.Close()

	// See also: https://stackoverflow.com/questions/36921190/difference-between-http-and-default-servemux
	mux := http.NewServeMux() // `Mux` stands for multiplexer
	mux.HandleFunc("/polls/", withCORS(withVars(withData(db, withAPIKey(handlePolls)))))

	log.Println("Webサーバーを開始します:", *addr)
	graceful.Run(*addr, 1*time.Second, mux)
	log.Println("停止します...")
}

func withAPIKey(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isValidAPIKey(r.URL.Query().Get("key")) {
			respondErr(w, r, http.StatusUnauthorized, "不正なAPIキーです")
			return
		}

		fn(w, r)
	}
}

func isValidAPIKey(key string) bool {
	return key == "abc123"
}

func withData(d *mgo.Session, f http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		thisDb := d.Copy()
		defer thisDb.Close()

		SetVar(r, "db", thisDb.DB("ballots"))
		f(w, r)
	}
}

func withVars(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		OpenVars(r)
		defer CloseVars(r)

		fn(w, r)
	}
}

func withCORS(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Expose-Headers", "Location")

		fn(w, r)
	}
}
