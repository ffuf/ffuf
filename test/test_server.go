package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

// TestServer simulates a web application with predictable patterns for testing Markov Chain feedback
type TestServer struct {
	patterns map[string]int
}

// NewTestServer creates a new test server with predefined patterns
func NewTestServer() *TestServer {
	rand.Seed(time.Now().UnixNano())
	
	return &TestServer{
		patterns: map[string]int{
			"/admin":     200,
			"/login":     200,
			"/user":      200,
			"/api":       200,
			"/config":    200,
			"/settings":  200,
			"/dashboard": 200,
			// Common paths that return 404
			"/random1":   404,
			"/random2":   404,
			"/random3":   404,
			"/random4":   404,
			"/random5":   404,
			"/test123":   404,
			"/unknown":   404,
			// Some 403 forbidden paths
			"/forbidden": 403,
			"/private":   403,
		},
	}
}

// ServeHTTP handles incoming requests
func (ts *TestServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.ToLower(r.URL.Path)
	
	// Add some delay to create different response times
	delay := time.Duration(rand.Intn(100)) * time.Millisecond
	time.Sleep(delay)
	
	// Check if the path exists in our patterns
	if status, exists := ts.patterns[path]; exists {
		w.WriteHeader(status)
		
		switch status {
		case 200:
			// Different content for different 200 responses
			switch path {
			case "/admin":
				w.Header().Set("Content-Type", "text/html")
				fmt.Fprintf(w, "<html><head><title>Admin Panel</title></head><body><h1>Admin Dashboard</h1><p>Welcome to the admin panel</p></body></html>")
			case "/login":
				w.Header().Set("Content-Type", "text/html")
				fmt.Fprintf(w, "<html><head><title>Login</title></head><body><form><input type='text' name='username'><input type='password' name='password'></form></body></html>")
			case "/user":
				w.Header().Set("Content-Type", "text/html")
				fmt.Fprintf(w, "<html><head><title>User Profile</title></head><body><h2>User Profile Page</h2><p>Profile information here</p></body></html>")
			case "/api":
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"status":"success", "data":"API response", "version":"1.0"}`)
			case "/config":
				w.Header().Set("Content-Type", "text/plain")
				fmt.Fprintf(w, "database_url=example.com\ndb_user=admin\nsecret_key=xyz123")
			case "/settings":
				w.Header().Set("Content-Type", "text/html")
				fmt.Fprintf(w, "<html><head><title>Settings</title></head><body><h1>System Settings</h1><form>Various settings</form></body></html>")
			case "/dashboard":
				w.Header().Set("Content-Type", "text/html")
				fmt.Fprintf(w, "<html><head><title>Dashboard</title></head><body><h1>Dashboard</h1><div>Welcome to dashboard</div></body></html>")
			}
		case 403:
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, "<html><head><title>Forbidden</title></head><body><h1>403 Forbidden</h1><p>You don't have permission to access this resource.</p></body></html>")
		default:
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, "<html><head><title>Not Found</title></head><body><h1>404 Not Found</h1><p>The requested URL was not found on this server.</p></body></html>")
		}
	} else {
		// For paths not in our pattern, return 404
		w.WriteHeader(404)
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, "<html><head><title>Not Found</title></head><body><h1>404 Not Found</h1><p>The requested URL was not found on this server.</p></body></html>")
	}
}

func main() {
	server := NewTestServer()
	
	fmt.Println("Starting test server on :8080")
	fmt.Println("Available paths with 200 responses:")
	for path, status := range server.patterns {
		if status == 200 {
			fmt.Printf("  http://localhost:8080%s\n", path)
		}
	}
	fmt.Println("\nAvailable paths with 403 responses:")
	for path, status := range server.patterns {
		if status == 403 {
			fmt.Printf("  http://localhost:8080%s\n", path)
		}
	}
	fmt.Println("\nAll other paths return 404")
	fmt.Println("\nRun the following command to test Markov Chain feedback:")
	fmt.Println("ffuf -w test_wordlist.txt -u http://localhost:8080/FUZZ -mc 200,403 -t 50")
	
	err := http.ListenAndServe(":8080", server)
	if err != nil {
		log.Fatal("Server failed to start: ", err)
	}
}