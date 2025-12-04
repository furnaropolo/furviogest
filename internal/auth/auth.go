package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"furviogest/internal/database"
	"furviogest/internal/models"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("credenziali non valide")
	ErrUserNotFound       = errors.New("utente non trovato")
	ErrUserInactive       = errors.New("utente non attivo")
)

// Session rappresenta una sessione utente
type Session struct {
	Token     string
	UserID    int64
	Username  string
	Ruolo     models.Ruolo
	Nome      string
	Cognome   string
	Email     string
	ExpiresAt time.Time
}

// SessionStore gestisce le sessioni attive
type SessionStore struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

var Sessions = &SessionStore{
	sessions: make(map[string]*Session),
}

// HashPassword genera l'hash della password
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPassword verifica se la password corrisponde all'hash
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateToken genera un token di sessione casuale
func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// Login esegue il login e crea una sessione
func Login(username, password string) (*Session, error) {
	var user models.Utente
	err := database.DB.QueryRow(`
		SELECT id, username, password, nome, cognome, email, ruolo, attivo
		FROM utenti WHERE username = ?
	`, username).Scan(
		&user.ID, &user.Username, &user.Password,
		&user.Nome, &user.Cognome, &user.Email,
		&user.Ruolo, &user.Attivo,
	)

	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if !user.Attivo {
		return nil, ErrUserInactive
	}

	if !CheckPassword(password, user.Password) {
		return nil, ErrInvalidCredentials
	}

	token, err := GenerateToken()
	if err != nil {
		return nil, err
	}

	session := &Session{
		Token:     token,
		UserID:    user.ID,
		Username:  user.Username,
		Ruolo:     user.Ruolo,
		Nome:      user.Nome,
		Cognome:   user.Cognome,
		Email:     user.Email,
		ExpiresAt: time.Now().Add(24 * time.Hour), // Sessione valida 24 ore
	}

	Sessions.Set(token, session)
	return session, nil
}

// Logout rimuove la sessione
func Logout(token string) {
	Sessions.Delete(token)
}

// Set aggiunge una sessione
func (s *SessionStore) Set(token string, session *Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[token] = session
}

// Get recupera una sessione
func (s *SessionStore) Get(token string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[token]
	if !ok {
		return nil, false
	}
	if time.Now().After(session.ExpiresAt) {
		go s.Delete(token) // Rimuovi sessione scaduta
		return nil, false
	}
	return session, true
}

// Delete rimuove una sessione
func (s *SessionStore) Delete(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, token)
}

// CleanExpired rimuove tutte le sessioni scadute
func (s *SessionStore) CleanExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for token, session := range s.sessions {
		if now.After(session.ExpiresAt) {
			delete(s.sessions, token)
		}
	}
}

// StartCleanupRoutine avvia la routine di pulizia delle sessioni scadute
func StartCleanupRoutine() {
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		for range ticker.C {
			Sessions.CleanExpired()
		}
	}()
}

// CreateUser crea un nuovo utente
func CreateUser(username, password, nome, cognome, email, telefono string, ruolo models.Ruolo) (*models.Utente, error) {
	hashedPassword, err := HashPassword(password)
	if err != nil {
		return nil, err
	}

	result, err := database.DB.Exec(`
		INSERT INTO utenti (username, password, nome, cognome, email, telefono, ruolo, attivo)
		VALUES (?, ?, ?, ?, ?, ?, ?, 1)
	`, username, hashedPassword, nome, cognome, email, telefono, ruolo)

	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()
	return &models.Utente{
		ID:       id,
		Username: username,
		Nome:     nome,
		Cognome:  cognome,
		Email:    email,
		Telefono: telefono,
		Ruolo:    ruolo,
		Attivo:   true,
	}, nil
}

// UpdatePassword aggiorna la password di un utente
func UpdatePassword(userID int64, newPassword string) error {
	hashedPassword, err := HashPassword(newPassword)
	if err != nil {
		return err
	}

	_, err = database.DB.Exec(`
		UPDATE utenti SET password = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?
	`, hashedPassword, userID)

	return err
}

// GetUserByID recupera un utente per ID
func GetUserByID(id int64) (*models.Utente, error) {
	var user models.Utente
	err := database.DB.QueryRow(`
		SELECT id, username, nome, cognome, email, telefono, ruolo, attivo, documento_path, created_at, updated_at
		FROM utenti WHERE id = ?
	`, id).Scan(
		&user.ID, &user.Username, &user.Nome, &user.Cognome,
		&user.Email, &user.Telefono, &user.Ruolo, &user.Attivo,
		&user.DocumentoPath, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// IsTecnico verifica se l'utente è un tecnico
func (s *Session) IsTecnico() bool {
	return s.Ruolo == models.RuoloTecnico
}

// IsGuest verifica se l'utente è un guest
// IsAmministrazione verifica se l utente e della contabilita
func (s *Session) IsAmministrazione() bool {
	return s.Ruolo == models.RuoloAmministrazione
}

func (s *Session) IsGuest() bool {
	return s.Ruolo == models.RuoloGuest
}

// NomeCompleto restituisce il nome completo dell'utente
func (s *Session) NomeCompleto() string {
	return s.Nome + " " + s.Cognome
}
