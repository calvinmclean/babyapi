package main

import (
	"github.com/calvinmclean/babyapi"
	"github.com/calvinmclean/babyapi/storage"
	"github.com/madflojo/hord/drivers/hashmap"
)

type User struct {
	babyapi.DefaultResource
	FirstName string
	LastName  string
}

func main() {
	api := babyapi.NewAPI[*User](
		"Users", "/users",
		func() *User { return &User{} },
	)

	db, err := storage.NewFileDB(hashmap.Config{
		Filename: "storage.json",
	})
	if err != nil {
		panic(err)
	}

	api.SetStorage(storage.NewClient[*User](db, "User"))

	api.RunCLI()
}
