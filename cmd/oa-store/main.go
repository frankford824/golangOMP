package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"workflow/internal/oa/objectstore"
)

func main() {
	root, secret := os.Getenv("OA_OBJECT_ROOT"), os.Getenv("OA_OBJECT_SECRET")
	if !filepath.IsAbs(root) || len(secret) < 32 {
		log.Fatal("set an absolute OA_OBJECT_ROOT and OA_OBJECT_SECRET (32 characters minimum)")
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		log.Fatal(err)
	}
	server := &http.Server{Addr: "127.0.0.1:18088", Handler: objectstore.New(root, "oa-validation", secret), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 5 * time.Minute, WriteTimeout: 6 * time.Minute, IdleTimeout: 60 * time.Second}
	log.Fatal(server.ListenAndServe())
}
