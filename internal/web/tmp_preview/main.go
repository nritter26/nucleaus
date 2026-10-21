package main

import (
	"log"
	"net/http"

	"nucleaus/internal/web"
)

func main() {
	mux := http.NewServeMux()
	web.RegisterStatic(mux)
	log.Fatal(http.ListenAndServe(":8099", mux))
}
