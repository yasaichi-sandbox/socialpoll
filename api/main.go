package main

import (
	"net/http"
)

func main() {}

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

func respondErr(w http.ResponseWriter, r *http.Request, status int, args ...interface{}) {
	// TODO: Implement later
}
