package main

import (
	"context"
	"log"
	"os"

	"github.com/senior-sigan/prettylog/internal"
)

func main() {
	log.SetFlags(0)

	log.Printf("reading stdin...")

	ctx := context.Background()
	if err := internal.Scan(ctx, os.Stdin); err != nil {
		log.Fatalf("scanning caught an error: %v", err)
	}
}
