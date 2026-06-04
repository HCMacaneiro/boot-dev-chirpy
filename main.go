package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/HCMacaneiro/boot-dev-chirpy/internal/database"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}

type apiConfig struct {
	fileserverHits  atomic.Int32
	databaseQueries *database.Queries
}

type Chirp struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserID    uuid.UUID `json:"user_id"`
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) countNumOfReqs(w http.ResponseWriter, r *http.Request) {
	fileserverHits := cfg.fileserverHits.Load()
	msg := fmt.Sprintf(`
	<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
</html>
	`, fileserverHits)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(msg))

}

func (cfg *apiConfig) resetMetrics(w http.ResponseWriter, r *http.Request) {
	godotenv.Load(".env")
	dbPlatform := os.Getenv("PLATFORM")
	if dbPlatform != "dev" {
		respondWithError(w, 403, "Forbidden")
		return
	}
	cfg.fileserverHits.Swap(0)
	err := cfg.databaseQueries.DeleteAllUsers(r.Context())
	if err != nil {
		respondWithError(w, 500, "Something went wrong")
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)

}

func respondWithError(w http.ResponseWriter, code int, msg string) {
	type returnErr struct {
		Error string `json:"error"`
	}

	payload := returnErr{Error: msg}

	respondWithJSON(w, code, payload)

}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshaling JSON: %s", err)
		w.WriteHeader(500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(data)

}

func (cfg *apiConfig) handleChirp(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body   string    `json:"body"`
		UserId uuid.UUID `json:"user_id"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, 500, "Something went wrong")
		return
	}
	if len(params.Body) > 140 {
		respondWithError(w, 400, "Chirp is too long")
		return
	}

	newBody := censorBadWords(params.Body)

	type payload struct {
		Body   string    `json:"body"`
		UserID uuid.UUID `json:"user_id"`
	}

	resp := payload{Body: newBody, UserID: params.UserId}

	chirp, err := cfg.databaseQueries.AddChirp(r.Context(), database.AddChirpParams{
		Body:   resp.Body,
		UserID: resp.UserID,
	})

	formattedChirp := Chirp(chirp)

	respondWithJSON(w, 201, formattedChirp)

}

func censorBadWords(msg string) string {
	words := strings.Split(msg, " ")
	cleanedWords := []string{}
	for _, word := range words {
		if strings.ToLower(word) == "kerfuffle" || strings.ToLower(word) == "sharbert" || strings.ToLower(word) == "fornax" {
			word = "****"
		}
		cleanedWords = append(cleanedWords, word)
	}
	return strings.Join(cleanedWords, " ")
}

func (cfg *apiConfig) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Email string `json:"email"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, 500, "Something went wrong")
		return
	}
	newUser, err := cfg.databaseQueries.CreateUser(r.Context(), params.Email)
	if err != nil {
		respondWithError(w, 500, "Something went wrong")
		return
	}

	respVal := User(newUser)

	respondWithJSON(w, 201, respVal)

}

func (cfg *apiConfig) handleGetAllChirps(w http.ResponseWriter, r *http.Request) {
	chirps, err := cfg.databaseQueries.GetAllChirps(r.Context())
	if err != nil {
		respondWithError(w, 500, "Something went wrong")
		return
	}
	jsonChirps := []Chirp{}
	for _, chirp := range chirps {
		jsonChirps = append(jsonChirps, Chirp(chirp))
	}

	respondWithJSON(w, 200, jsonChirps)

}

func (cfg *apiConfig) handleGetChirp(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("chirpID")
	chirpID, err := uuid.Parse(idStr)
	if err != nil {
		respondWithError(w, 500, "Something went wrong")
		return
	}
	chirp, err := cfg.databaseQueries.GetChirp(r.Context(), chirpID)
	if err != nil {
		respondWithError(w, 404, "Not found")
		return
	}

	jsonChirp := Chirp(chirp)
	respondWithJSON(w, 200, jsonChirp)

}

func main() {
	godotenv.Load(".env")
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal("Something went wrong")
	}

	const port = "8080"
	const filePathRoot = "."
	api := &apiConfig{}
	api.databaseQueries = database.New(db)

	mux := http.NewServeMux()
	mux.Handle("/app/", http.StripPrefix("/app/", api.middlewareMetricsInc(http.FileServer(http.Dir(".")))))

	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		msg := []byte("OK")
		w.Write(msg)

	})

	mux.HandleFunc("GET /admin/metrics", api.countNumOfReqs)
	mux.HandleFunc("POST /admin/reset", api.resetMetrics)
	mux.HandleFunc("POST /api/chirps", api.handleChirp)
	mux.HandleFunc("POST /api/users", api.handleCreateUser)
	mux.HandleFunc("GET /api/chirps", api.handleGetAllChirps)
	mux.HandleFunc("GET /api/chirps/{chirpID}", api.handleGetChirp)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	log.Fatal(srv.ListenAndServe())

}
