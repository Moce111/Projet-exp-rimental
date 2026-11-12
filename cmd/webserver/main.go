package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type submission struct {
	Session string                   `json:"session"`
	Profile map[string]string        `json:"profile"`
	Rows    []map[string]interface{} `json:"rows"`
}

var writeMu sync.Mutex

func results(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var sub submission
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20)).Decode(&sub); err != nil || sub.Session == "" || len(sub.Rows) == 0 {
		http.Error(w, "invalid submission", http.StatusBadRequest)
		return
	}
	writeMu.Lock()
	defer writeMu.Unlock()
	if err := os.MkdirAll("data", 0750); err != nil {
		http.Error(w, "storage unavailable", 500)
		return
	}
	file := filepath.Join("data", "results.csv")
	newFile := false
	if info, err := os.Stat(file); err == nil {
		newFile = info.Size() == 0
	} else if os.IsNotExist(err) {
		newFile = true
	} else {
		http.Error(w, "storage unavailable", 500)
		return
	}
	f, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		http.Error(w, "storage unavailable", 500)
		return
	}
	defer f.Close()
	c := csv.NewWriter(f)
	if newFile {
		_ = c.Write([]string{"received_at", "session", "phase", "trial", "continuum", "response_key", "response", "rt_ms", "correct", "rating_1to5", "payload_json"})
	}
	for _, row := range sub.Rows {
		payload, _ := json.Marshal(row)
		get := func(k string) string {
			if v, ok := row[k]; ok {
				return fmt.Sprint(v)
			}
			return ""
		}
		if err := c.Write([]string{time.Now().UTC().Format(time.RFC3339), sub.Session, get("phase"), get("trial"), get("continuum"), get("response_key"), get("response"), get("rt_ms"), get("correct"), get("rating_1to5"), string(payload)}); err != nil {
			http.Error(w, "write failed", 500)
			return
		}
	}
	c.Flush()
	if err := c.Error(); err != nil {
		http.Error(w, "write failed", 500)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	http.Handle("/", http.FileServer(http.Dir("web")))
	http.HandleFunc("/api/results", results)
	log.Printf("Perception musicale disponible sur http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
