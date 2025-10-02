package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	crdberrors "github.com/cockroachdb/errors"
	"github.com/kis9a/cockroachdb-errors-example/domain"
	"github.com/kis9a/cockroachdb-errors-example/logx"
)

// User represents a user entity
type User struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

// UserService simulates a user service with database operations
type UserService struct {
	users map[int]*User
}

// NewUserService creates a new user service
func NewUserService() *UserService {
	return &UserService{
		users: map[int]*User{
			1: {ID: 1, Name: "Alice", Email: "alice@example.com", CreatedAt: time.Now()},
			2: {ID: 2, Name: "Bob", Email: "bob@example.com", CreatedAt: time.Now()},
			3: {ID: 3, Name: "Charlie", Email: "charlie@example.com", CreatedAt: time.Now()},
		},
	}
}

// GetUser fetches a user by ID
func (s *UserService) GetUser(id int) (*User, error) {
	// Simulate temporary database connection issues (10% of requests)
	if time.Now().Unix()%10 == 0 {
		err := crdberrors.New("database connection timeout")
		err = domain.MarkTemporary(err)
		err = crdberrors.WithDomain(err, domain.DomainAdapters)
		err = crdberrors.WithHint(err, "Retry the request")

		return nil, domain.WrapWithStack(err, "failed to fetch user from database")
	}

	user, ok := s.users[id]
	if !ok {
		err := crdberrors.Errorf("user with id %d not found", id)
		err = crdberrors.WithDomain(err, domain.DomainAdapters)
		err = domain.MarkPermanent(err)

		return nil, err
	}

	return user, nil
}

// CreateUser creates a new user
func (s *UserService) CreateUser(name, email string) (*User, error) {
	// Validate input
	if name == "" {
		err := crdberrors.New("name is required")
		err = crdberrors.WithDomain(err, domain.DomainUsecase)
		err = domain.MarkPermanent(err)
		err = crdberrors.WithHint(err, "Provide a valid name")

		return nil, err
	}

	if email == "" {
		err := crdberrors.New("email is required")
		err = crdberrors.WithDomain(err, domain.DomainUsecase)
		err = domain.MarkPermanent(err)
		err = crdberrors.WithHint(err, "Provide a valid email address")

		return nil, err
	}

	// Generate new ID
	newID := len(s.users) + 1

	user := &User{
		ID:        newID,
		Name:      name,
		Email:     email,
		CreatedAt: time.Now(),
	}

	s.users[newID] = user
	return user, nil
}

// APIServer represents the HTTP API server
type APIServer struct {
	userService *UserService
}

// NewAPIServer creates a new API server
func NewAPIServer() *APIServer {
	return &APIServer{
		userService: NewUserService(),
	}
}

// respondJSON sends a JSON response
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		logx.ErrorErr("Failed to encode JSON response", err)
	}
}

// respondError sends an error response with proper logging
func respondError(w http.ResponseWriter, status int, err error, requestID string) {
	// Log error with full context
	logx.ErrorErr("API request failed", err,
		"request_id", requestID,
		"status", status,
	)

	// Prepare error response
	errorResp := ErrorResponse{
		Error: err.Error(),
	}

	// Add domain-specific information if available
	if errorDomain := crdberrors.GetDomain(err); errorDomain != crdberrors.NoDomain {
		errorResp.Code = fmt.Sprintf("%v", errorDomain)
	}

	// Add hints for client
	if hints := crdberrors.GetAllHints(err); len(hints) > 0 {
		errorResp.Details = hints[0]
	}

	respondJSON(w, status, errorResp)
}

// getUserHandler handles GET /users/:id
func (s *APIServer) getUserHandler(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	if requestID == "" {
		requestID = fmt.Sprintf("req_%d", time.Now().UnixNano())
	}
	ctx := context.WithValue(r.Context(), "request_id", requestID)

	// Extract user ID from URL
	idStr := strings.Trim(strings.TrimPrefix(r.URL.Path, "/users/"), "/")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		err = crdberrors.Wrap(err, "invalid user ID")
		err = domain.MarkPermanent(err)
		respondError(w, http.StatusBadRequest, err, requestID)
		return
	}

	logx.WithContext(ctx).Info("Fetching user",
		"request_id", requestID,
		"user_id", id,
	)

	// Fetch user from service
	user, err := s.userService.GetUser(id)
	if err != nil {
		// Determine HTTP status based on error type
		status := http.StatusInternalServerError
		if domain.IsPermanent(err) {
			status = http.StatusNotFound
		}

		respondError(w, status, err, requestID)
		return
	}

	logx.WithContext(ctx).Info("User fetched successfully",
		"request_id", requestID,
		"user_id", id,
	)

	respondJSON(w, http.StatusOK, user)
}

// createUserHandler handles POST /users
func (s *APIServer) createUserHandler(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	if requestID == "" {
		requestID = fmt.Sprintf("req_%d", time.Now().UnixNano())
	}
	ctx := context.WithValue(r.Context(), "request_id", requestID)

	// Parse request body
	// Limit body size to 1MB
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	var req struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := dec.Decode(&req); err != nil {
		err = crdberrors.Wrap(err, "invalid JSON request")
		err = domain.MarkPermanent(err)
		respondError(w, http.StatusBadRequest, err, requestID)
		return
	}

	// Extra tokens? reject.
	if dec.More() {
		err := crdberrors.New("extraneous data after JSON object")
		err = domain.MarkPermanent(err)
		respondError(w, http.StatusBadRequest, err, requestID)
		return
	}

	logx.WithContext(ctx).Info("Creating user",
		"request_id", requestID,
		"name", req.Name,
		"email", req.Email,
	)

	// Create user
	user, err := s.userService.CreateUser(req.Name, req.Email)
	if err != nil {
		status := http.StatusInternalServerError
		if domain.IsPermanent(err) {
			status = http.StatusBadRequest
		}

		respondError(w, status, err, requestID)
		return
	}

	logx.WithContext(ctx).Info("User created successfully",
		"request_id", requestID,
		"user_id", user.ID,
	)

	respondJSON(w, http.StatusCreated, user)
}

// healthHandler handles GET /health
func (s *APIServer) healthHandler(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	})
}

// Routes sets up HTTP routes
func (s *APIServer) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", s.healthHandler)
	mux.HandleFunc("/users/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			s.getUserHandler(w, r)
		} else {
			respondJSON(w, http.StatusMethodNotAllowed, ErrorResponse{
				Error: "method not allowed",
			})
		}
	})
	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			s.createUserHandler(w, r)
		} else {
			respondJSON(w, http.StatusMethodNotAllowed, ErrorResponse{
				Error: "method not allowed",
			})
		}
	})

	return mux
}

func main() {
	fmt.Println("Starting HTTP API server with error handling demo")
	fmt.Println("=================================================")

	server := NewAPIServer()

	addr := ":8888"
	fmt.Printf("\nServer listening on %s\n\n", addr)

	fmt.Println("Test the API with curl:")
	fmt.Println("  Health check:")
	fmt.Println("    curl http://localhost:8888/health")
	fmt.Println("\n  Get user (success):")
	fmt.Println("    curl http://localhost:8888/users/1")
	fmt.Println("\n  Get user (not found):")
	fmt.Println("    curl http://localhost:8888/users/999")
	fmt.Println("\n  Get user (invalid ID):")
	fmt.Println("    curl http://localhost:8888/users/abc")
	fmt.Println("\n  Create user (success):")
	fmt.Println("    curl -X POST http://localhost:8888/users -H 'Content-Type: application/json' -d '{\"name\":\"David\",\"email\":\"david@example.com\"}'")
	fmt.Println("\n  Create user (validation error):")
	fmt.Println("    curl -X POST http://localhost:8888/users -H 'Content-Type: application/json' -d '{\"name\":\"\",\"email\":\"\"}'")
	fmt.Println()

	// Start server
	if err := http.ListenAndServe(addr, server.Routes()); err != nil {
		logx.ErrorErr("Server failed to start", err)
	}
}
