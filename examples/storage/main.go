package main

import (
	"github.com/calvinmclean/babyapi"
	"github.com/calvinmclean/babyapi/storage/kv"
	"github.com/tarmac-project/hord/drivers/hashmap"
)

type User struct {
	babyapi.DefaultResource
	FirstName string
	LastName  string
}

func main() {
	api := babyapi.NewAPI(
		"Users", "/users",
		func() *User { return &User{} },
	)

	db, err := kv.NewFileDB(hashmap.Config{
		Filename: "storage.json",
	})
	if err != nil {
		panic(err)
	}

	api.SetStorage(babyapi.NewKVStorage[*User](db, "User"))

	api.RunCLI()
}
