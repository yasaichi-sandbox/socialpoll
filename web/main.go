package main

import (
	"flag"
	"log"
	"net/http"
)

func main() {
	addr := flag.String("addr", ":8081", "Webサイトのアドレス")
	flag.Parse()

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir("public")))

	log.Println("Webサイトのアドレス:", *addr)
	// NOTE: *http.ServeMux implements `ServeHTTP` required by http.Handler
	http.ListenAndServe(*addr, mux)
}
