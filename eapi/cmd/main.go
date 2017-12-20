package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"../../eapi"
	"github.com/dgrijalva/jwt-go"
)

const tokenTimeToLive = 6

type database struct {
	sync.RWMutex
	mapa map[string]time.Time
}

type account struct {
	Name     string `json:"user"`
	Password string `json:"password"`
}

func main() {
	inMemDB := newMap()

	http.HandleFunc("/hello", inMemDB.handlerTokenized)
	http.HandleFunc("/login", inMemDB.createToken)

	log.Fatal(http.ListenAndServe(":8080", nil))
}

func newMap() *database {
	var m = &database{}
	m.mapa = make(map[string]time.Time)
	return m
}

func (db *database) validateToken(token string) bool {
	if token == "" {
		return false
	}

	db.RLock()
	_, ok := db.mapa[token]
	db.RUnlock()

	return ok
}

func (db *database) handlerTokenized(w http.ResponseWriter, req *http.Request) {
	tokenFromRequest := req.Header.Get("Authentication")
	if !db.validateToken(tokenFromRequest) {
		w.WriteHeader(http.StatusUnauthorized)
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

func (db *database) createToken(w http.ResponseWriter, req *http.Request) {
	mySigningKey := []byte("secret")

	var receivedAccount account
	err := json.NewDecoder(req.Body).Decode(&receivedAccount)
	if err != nil {
		log.Println(err)
		//w.WriteHeader(http.StatusBadRequest)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	log.Println("parsed json - ", receivedAccount)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": receivedAccount.Name,
		"password": receivedAccount.Password,
	})
	tokenString, err := token.SignedString(mySigningKey)
	if err != nil {
		log.Println(err)
		//w.WriteHeader(http.StatusInternalServerError)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	toBeDestroyedAt := time.Now().Add(tokenTimeToLive * time.Second)
	db.Lock()
	db.mapa[tokenString] = toBeDestroyedAt
	db.Unlock()

	time.AfterFunc(tokenTimeToLive*time.Second, func() {
		db.Lock()
		if ttl, ok := db.mapa[tokenString]; ok {
			if time.Now().After(ttl) {
				delete(db.mapa, tokenString)
				log.Println("\t\t\t delete from map")
			}
		}
		db.Unlock()
	})

	tokenCreated := eapi.JwtToken{
		Token:      tokenString,
		TimeToLive: toBeDestroyedAt,
	}
	err = json.NewEncoder(w).Encode(tokenCreated)
	if err != nil {
		http.Error(w, "can't Marshal/Encode token"+err.Error(), http.StatusInternalServerError)
		return
	}
}
