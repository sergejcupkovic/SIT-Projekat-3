package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
)

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	usersServiceURL := os.Getenv("USERS_SERVICE_URL")
	if usersServiceURL == "" {
		usersServiceURL = "http://users-service:8081" 
	}

	usersTarget, err := url.Parse(usersServiceURL)
	if err != nil {
		log.Fatalf("Invalid USERS_SERVICE_URL: %v", err)
	}
	usersProxy := httputil.NewSingleHostReverseProxy(usersTarget)

	usersProxy.ModifyResponse = func(resp *http.Response) error {
		resp.Header.Set("Access-Control-Allow-Origin", "*")
		resp.Header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		resp.Header.Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		return nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/users/", func(w http.ResponseWriter, r *http.Request) {
		usersProxy.ServeHTTP(w, r)
	})

	port := os.Getenv("GATEWAY_PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("API Gateway running on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, corsMiddleware(mux)))
}
