package main

import (
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"photo-dashboard/internal/demo"
	"photo-dashboard/internal/webfs"
)

func main() {
	addr := ":8080"
	if v := os.Getenv("ADDR"); v != "" {
		addr = v
	}

	seed := filepath.Join(webfs.ModuleRoot(), "internal", "demo", "seed.json")
	if _, err := os.Stat(seed); err != nil {
		seed = demo.SeedPath()
	}
	store, err := demo.Load(seed)
	if err != nil {
		log.Fatal("load seed: ", err)
	}
	hub := demo.NewHub(store)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", hub.ServeWS)

	static, err := webfs.FS()
	if err != nil {
		log.Fatal("web fs: ", err)
	}
	mux.Handle("/", spaFileServer(static))

	log.Printf("Home HQ listening on http://localhost%s", addr)
	log.Printf("WebSocket at ws://localhost%s/ws", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func spaFileServer(root fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(root, path); err != nil {
			// fall through to FileServer for 404 on missing assets
		}
		fileServer.ServeHTTP(w, r)
	})
}
