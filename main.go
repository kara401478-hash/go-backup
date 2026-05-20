package main

import (
	"flag"
	"log"
	"os"

	"github.com/yourusername/go-backup/internal/server"
)

func main() {
	port := flag.Int("port", 8080, "ポート番号")
	flag.Parse()

	log.SetFlags(0)

	srv := server.New(*port)
	if err := srv.Start(); err != nil {
		log.Printf("❌ エラー: %v", err)
		os.Exit(1)
	}
}
