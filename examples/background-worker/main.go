package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/calvinmclean/babyapi"
)

// backgroundWorker will create a new resource every second using the API Client
// It will shutdown when the context is cancelled or the API stops
func backgroundWorker(ctx context.Context, api *babyapi.API[*babyapi.DefaultResource]) {
	ticker := time.NewTicker(1 * time.Second)
	for {
		select {
		case <-api.Done():
			fmt.Println("background worker received shutdown from API")
			return
		case <-ctx.Done():
			fmt.Println("background worker received shutdown from context")
			return
		case <-ticker.C:
			client := api.Client("http://localhost:8080")
			result, err := client.Post(ctx, &babyapi.DefaultResource{})
			if err != nil {
				fmt.Println("error creating resource:", err.Error())
				continue
			}

			fmt.Println("background worker created new resource with id:", result.Data.GetID())
		}
	}
}

func main() {
	api := babyapi.NewAPI("nothing", "/nothing", func() *babyapi.DefaultResource { return &babyapi.DefaultResource{} })

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		backgroundWorker(ctx, api)
		wg.Done()
	}()

	time.AfterFunc(5*time.Second, cancel)

	api.WithContext(ctx).RunCLI()

	wg.Wait()
	fmt.Println("API is shutdown successfully!")
}
