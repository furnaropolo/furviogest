package middleware

import (
	"context"
	"furviogest/internal/auth"
	"furviogest/internal/models"
	"net/http"
)

type contextKey string

const SessionKey contextKey = "session"

// GetSession recupera la sessione dal contesto della richiesta
func GetSession(r *http.Request) *auth.Session {
	session, ok := r.Context().Value(SessionKey).(*auth.Session)
	if !ok {
		return nil
	}
	return session
}

// RequireAuth middleware che richiede autenticazione
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_token")
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		session, ok := auth.Sessions.Get(cookie.Value)
		if !ok {
			// Rimuovi il cookie non valido
			http.SetCookie(w, &http.Cookie{
				Name:   "session_token",
				Value:  "",
				Path:   "/",
				MaxAge: -1,
			})
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// Aggiungi la sessione al contesto
		ctx := context.WithValue(r.Context(), SessionKey, session)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireTecnico middleware che richiede ruolo tecnico
func RequireTecnico(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session := GetSession(r)
		if session == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		if session.Ruolo != models.RuoloTecnico {
			http.Error(w, "Accesso non autorizzato. Solo i tecnici possono eseguire questa operazione.", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RequireGuest middleware che permette solo utenti non autenticati (per login/register)
func RequireGuest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_token")
		if err == nil {
			if _, ok := auth.Sessions.Get(cookie.Value); ok {
				http.Redirect(w, r, "/", http.StatusSeeOther)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// Logging middleware per il logging delle richieste
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Log semplice - può essere esteso
		next.ServeHTTP(w, r)
	})
}

// RequireAmministrazione middleware che richiede ruolo amministrazione
func RequireAmministrazione(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session := GetSession(r)
		if session == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		if session.Ruolo != models.RuoloAmministrazione {
			http.Error(w, "Accesso non autorizzato. Solo l'amministrazione puo' accedere.", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RequireTecnicoOrAmministrazione permette tecnici e amministrazione
func RequireTecnicoOrAmministrazione(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session := GetSession(r)
		if session == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		if session.Ruolo != models.RuoloTecnico && session.Ruolo != models.RuoloAmministrazione {
			http.Error(w, "Accesso non autorizzato.", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}
