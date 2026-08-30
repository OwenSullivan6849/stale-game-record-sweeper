package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"example.com/stale-game-record-sweeper/internal/infrai"
)

func main() {
	cronExpr := flag.String("cron", "17 * * * *", "cron expression for the cleanup sweep")
	flag.Parse()

	taskURL := os.Getenv("CLEANUP_TASK_URL")
	if taskURL == "" {
		log.Fatal("CLEANUP_TASK_URL is required")
	}

	client, err := infrai.New(os.Getenv("INFRAI_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}
	job, err := client.CreateCron(context.Background(), infrai.CronCreateRequest{
		CronExpr: *cronExpr,
		Task:     taskURL,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("cleanup sweep registered: %s\n", job.JobID)
}
