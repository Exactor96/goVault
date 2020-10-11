package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"
)

type vaultItem struct {
	Value     string `JSON:"value"`
	validTo   int64  `JSON:"validTo"`
	isExpired bool
}

func getValue(vi *vaultItem) string {
	return vi.Value
}

type requestJSON struct {
	Value   string `JSON:"value"`
	Key     string `JSON:"key"`
	validTo int64  `JSON:"validTo"`
}

type responseJSON struct {
	Success bool     `JSON:"success"`
	Errors  []string `JSON:"errors, omitempty"`
	Value   string   `JSON:"value, omitempty"`
}

//Vault - valut with pool
type Vault struct {
	vault    map[string]vaultItem
	jobsPool chan int
}

//Enviroment vars

//CleanerRunTimeout - timeout in seconds for cleaning expired items in vault, also checking items' expiration
var CleanerRunTimeout uint64

//DefaultExpiration - time seting as default expiration for items
var DefaultExpiration uint64

//PoolLimit - connections limit for vault
var PoolLimit uint64

var vault = Vault{}

func (v *Vault) init() {
	fmt.Println("INIT Vault")
	if os.Getenv("PoolLimit") == "" {
		fmt.Println("You need set variable `PoolLimit`", "Default value set: 10")
		PoolLimit = 10

	} else {
		PoolLimit, _ = strconv.ParseUint(os.Getenv("PoolLimit"), 10, 64)
	}

	v.jobsPool = make(chan int, PoolLimit)
}

func prepareRespone(r responseJSON) []byte {
	js, err := json.Marshal(r)
	if err != nil {
		js, _ := json.Marshal(responseJSON{Success: false, Errors: []string{err.Error()}})
		return js
	}
	return js
}

func init() {
	if os.Getenv("CleanerRunTimeout") == "" {
		fmt.Println("You need set variable `CleanerRunTimeout`", "Default value set: 60")
		CleanerRunTimeout = 60

	} else {
		CleanerRunTimeout, _ = strconv.ParseUint(os.Getenv("CleanerRunTimeout"), 10, 64)
	}
	if os.Getenv("DefaultExpiration") == "" {
		fmt.Println("You need set variable `DefaultExpiration`", "Default value set: 120")
		DefaultExpiration = 120

	} else {
		DefaultExpiration, _ = strconv.ParseUint(os.Getenv("DefaultExpiration"), 10, 64)
	}

	vault.init()

}

func mainVault(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vault.jobsPool <- 1

	if req.Method == "POST" {

		decoder := json.NewDecoder(req.Body)
		var t requestJSON
		err := decoder.Decode(&t)
		if err != nil {
			js := prepareRespone(responseJSON{Success: false, Errors: []string{err.Error()}})
			w.Write(js)
			<-vault.jobsPool
			return

		}

		if t.Key != "" && t.Value != "" {

			vault.vault[t.Key] = vaultItem{Value: t.Value, validTo: t.validTo}
			js := prepareRespone(responseJSON{Success: true, Errors: []string{}})
			w.Write(js)

		} else {

			js := prepareRespone(responseJSON{Success: false, Errors: []string{}})
			w.Write(js)
		}
		<-vault.jobsPool
		return
	}
	if req.Method == "GET" {
		keys, ok := req.URL.Query()["key"]

		if !ok || len(keys[0]) < 1 {
			js := prepareRespone(responseJSON{Success: false, Errors: []string{"key is missing"}})
			w.Write(js)
			<-vault.jobsPool
			return
		}

		if keys[0] != "" {
			val := vault.vault[keys[0]]
			if val.Value == "" {
				js := prepareRespone(responseJSON{Success: false, Errors: []string{"value not found"}})
				w.Write(js)
				<-vault.jobsPool
				return

			} else {
				if val.validTo < time.Now().Unix() {
					js := prepareRespone(responseJSON{Success: false, Errors: []string{"items is expired"}})
					w.Write(js)
					val.isExpired = true
					vault.vault[keys[0]] = val
					<-vault.jobsPool
					return
				} else {
					js := prepareRespone(responseJSON{Success: true, Value: val.Value})
					w.Write(js)
				}
			}

		}
		<-vault.jobsPool
		return
	}

	if req.Method == "DELETE" {

		keys, ok := req.URL.Query()["key"]

		if !ok || len(keys[0]) < 1 {
			js := prepareRespone(responseJSON{Success: false, Errors: []string{"key is missing"}})
			w.Write(js)
			<-vault.jobsPool
			return
		}

		if keys[0] != "" {
			val := vault.vault[keys[0]]
			if val.Value == "" {
				js := prepareRespone(responseJSON{Success: false, Errors: []string{"value not found"}})
				w.Write(js)

			} else {
				delete(vault.vault, keys[0])
				js := prepareRespone(responseJSON{Success: true})
				w.Write(js)
			}

		}
		<-vault.jobsPool
		return
	}
}

func main() {

	http.HandleFunc("/vault", mainVault)
	go cleanVault()
	http.ListenAndServe(":8888", nil)
}

func cleanVault() {

	for {

		time.Sleep(time.Duration(CleanerRunTimeout) * time.Second)

		for key, item := range vault.vault {
			if item.isExpired {
				delete(vault.vault, key)
			}
			if item.validTo < time.Now().Unix() {
				delete(vault.vault, key)

			}
		}
	}
}
