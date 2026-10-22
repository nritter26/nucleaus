package user

import (
	"net/http"

	"nucleaus/internal/web"

	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID   int
	Name string
}

type handler struct{ pool *pgxpool.Pool }

func RegisterRoutes(mux *http.ServeMux, pool *pgxpool.Pool) {
	h := &handler{pool: pool}
	mux.HandleFunc("GET /{$}", h.index)
	mux.HandleFunc("POST /users", h.create)
}

func (h *handler) index(w http.ResponseWriter, r *http.Request) {
	rows, err := h.pool.Query(r.Context(), `SELECT id, name FROM users ORDER BY id`)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Name); err != nil {
			http.Error(w, "db error", 500)
			return
		}
		users = append(users, u)
	}
	web.Render(w, "users_page", map[string]any{"Title": "Users", "Users": users})
}

func (h *handler) create(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}
	var u User
	err := h.pool.QueryRow(r.Context(),
		`INSERT INTO users (name) VALUES ($1) RETURNING id, name`, name).Scan(&u.ID, &u.Name)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	web.Render(w, "user_row", u) // htmx fragment
}
