package main

import (
	"net/http"
	"sync"
)

var (
	varsLock sync.RWMutex
	vars     map[*http.Request]map[string]interface{}
)

func OpenVars(r *http.Request) {
	varsLock.Lock()
	defer varsLock.Unlock()

	if vars == nil {
		vars = map[*http.Request]map[string]interface{}{}
	}

	vars[r] = map[string]interface{}{}
}

func CloseVars(r *http.Request) {
	varsLock.Lock()
	defer varsLock.Unlock()

	delete(vars, r)
}

func GetVar(r *http.Request, key string) interface{} {
	varsLock.RLock()
	defer varsLock.RUnlock()

	return vars[r][key]
}

func SetVar(r *http.Request, key string, value interface{}) {
	varsLock.Lock()
	defer varsLock.Unlock()

	vars[r][key] = value
}
