package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	svix "github.com/svix/svix-webhooks/go"
)

type webhookHandler struct {
	db       *pgxpool.Pool
	verifier *svix.Webhook
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func nullableTimestamptz(sec *int64) any {
	if sec == nil {
		return nil
	}
	return time.Unix(*sec, 0).UTC()
}

// createUser handles user.created — plain INSERT only.
func (h *webhookHandler) createUser(ctx context.Context, u clerkUser) error {  
 tx, err := h.db.Begin(ctx)  
 if err != nil {  
  return err  
 }  
 defer tx.Rollback(ctx)  
  
   const insertUser = `  
      INSERT INTO users (id, first_name, last_name, username, image_url, gender, birthday,  
      password_enabled, two_factor_enabled, primary_email_address_id, primary_phone_number_id,  
      created_at, updated_at, last_sign_in_at)  
      VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,to_timestamp($12 / 1000.0),to_timestamp($13 / 1000.0),$14)`  
  
   if _, err := tx.Exec(ctx, insertUser,  
    u.ID, u.FirstName, u.LastName, u.Username, u.ImageURL, u.Gender, u.Birthday,  
    u.PasswordEnabled, u.TwoFactorEnabled, u.PrimaryEmailAddressID, u.PrimaryPhoneNumberID,  
    u.CreatedAt, u.UpdatedAt, nullableTimestamptz(u.LastSignInAt),  
   ); err != nil {  
  
    return fmt.Errorf("create user %s: %w", u.ID, err)  
  
   }  
   if err := tx.Commit(ctx); err != nil {  
    return err  
   }  
   return nil  
}

// updateUser handles user.updated — plain UPDATE only.
func (h *webhookHandler) updateUser(ctx context.Context, u clerkUser) error {  
 tx, err := h.db.Begin(ctx)  
 if err != nil {  
  return err  
 }  
 defer tx.Rollback(ctx)  
  
   const updateUser = `  
      UPDATE users SET first_name=$2, last_name=$3, username=$4, image_url=$5, gender=$6, birthday=$7,  
      password_enabled=$8, two_factor_enabled=$9, primary_email_address_id=$10, primary_phone_number_id=$11,updated_at=to_timestamp($12 / 1000.0), last_sign_in_at=$13  
      WHERE id=$1 AND updated_at < to_timestamp($13 / 1000.0)`  
  
   if _, err := tx.Exec(ctx, updateUser,  
    u.ID, u.FirstName, u.LastName, u.Username, u.ImageURL, u.Gender, u.Birthday,  
    u.PasswordEnabled, u.TwoFactorEnabled, u.PrimaryEmailAddressID, u.PrimaryPhoneNumberID,  
    u.UpdatedAt, nullableTimestamptz(u.LastSignInAt),  
   ); err != nil {  
    return fmt.Errorf("update user %s: %w", u.ID, err)  
   }  
   if err := tx.Commit(ctx); err != nil {  
    return err  
   }  
   return nil  
}

// deleteUser handles user.deleted.
func (h *webhookHandler) deleteUser(ctx context.Context, userID string) error {
	tx, err := h.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM users WHERE id=$1`, userID); err != nil {
		return fmt.Errorf("delete user %s: %w", userID, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

// handleClerk is POST /webhook.
func (h *webhookHandler) handleClerk(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MiB cap
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"status": "error", "message": "cannot read payload"})
		return
	}

	if err := h.verifier.Verify(payload, r.Header); err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"status": "error", "message": "invalid webhook signature"})
		return
	}

	var event clerkEvent
	if err := json.Unmarshal(payload, &event); err != nil || event.Type == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"status": "error", "message": "invalid event envelope"})
		return
	}

	switch event.Type {
	case "user.created":
		var u clerkUser
		if err := json.Unmarshal(event.Data, &u); err != nil || u.ID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"status": "error", "message": "invalid user payload"})
			return
		}
		if err := h.createUser(r.Context(), u); err != nil {
			log.Printf("webhook %s %s failed: %v", event.Type, u.ID, err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"status": "error", "message": "failed to persist user"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "type": event.Type, "user": u.ID})
	case "user.updated":
		var u clerkUser
		if err := json.Unmarshal(event.Data, &u); err != nil || u.ID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"status": "error", "message": "invalid user payload"})
			return
		}
		if err := h.updateUser(r.Context(), u); err != nil {
			log.Printf("webhook %s %s failed: %v", event.Type, u.ID, err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"status": "error", "message": "failed to update user"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "type": event.Type, "user": u.ID})
	case "user.deleted":
		var u clerkUser
		if err := json.Unmarshal(event.Data, &u); err != nil || u.ID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"status": "error", "message": "invalid user payload"})
			return
		}
		if err := h.deleteUser(r.Context(), u.ID); err != nil {
			log.Printf("webhook %s %s failed: %v", event.Type, u.ID, err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"status": "error", "message": "failed to delete user"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "type": event.Type, "user": u.ID})
	default:
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored", "type": event.Type})
	}
}

func main() {
	ctx := context.Background()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}
	secret := os.Getenv("CLERK_WEBHOOK_SECRET")
	if secret == "" {
		log.Fatal("CLERK_WEBHOOK_SECRET is not set")
	}

	db, err := connectDB(ctx, dbURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer db.Close()
	if err := ensureSchema(ctx, db); err != nil {
		log.Fatalf("schema: %v", err)
	}
	log.Println("db connected, schema ready")

	verifier, err := svix.NewWebhook(secret)
	if err != nil {
		log.Fatalf("svix: %v", err)
	}

	h := &webhookHandler{db: db, verifier: verifier}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		if err := db.Ping(r.Context()); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "down"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Post("/webhook", h.handleClerk)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("listening on :%s (POST /webhook)", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
