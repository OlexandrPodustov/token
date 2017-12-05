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
	go inMemDB.sanitizer()

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
	//check time to live of token
	db.RUnlock()

	return ok
}

func (db *database) handlerTokenized(w http.ResponseWriter, req *http.Request) {
	tokenFromRequest := req.Header.Get("Authentication")
	if !db.validateToken(tokenFromRequest) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (db *database) createToken(w http.ResponseWriter, req *http.Request) {
	var receivedAccount account
	mySigningKey := []byte("secret")

	err := json.NewDecoder(req.Body).Decode(&receivedAccount)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusBadRequest)
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
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	toBeDestroyedAt := time.Now().Add(tokenTimeToLive * time.Second)
	db.Lock()
	//log.Println("new token has been written to the map")
	db.mapa[tokenString] = toBeDestroyedAt
	db.Unlock()

	tokenCreated := eapi.JwtToken{
		Token:      tokenString,
		TimeToLive: toBeDestroyedAt,
	}
	err = json.NewEncoder(w).Encode(tokenCreated)
	if err != nil {
		http.Error(w, "can't Marshal/Encode token", http.StatusInternalServerError)
		return
	}
}

func (db *database) sanitizer() {
	for {
		time.Sleep(1 * time.Second)
		db.Lock()
		for key, value := range db.mapa {
			if time.Now().After(value) {
				log.Println(db.mapa)
				delete(db.mapa, key)
				log.Println(db.mapa)
			}
		}
		db.Unlock()
	}
}
