package email

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html/template"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"time"

	"furviogest/internal/models"
)

// SMTPConfig configurazione SMTP
type SMTPConfig struct {
	Server   string
	Port     int
	User     string
	Password string
	FromName string
	FromAddr string
}

// EmailData contiene i dati per l'email
type EmailData struct {
	To          []string
	Cc          []string
	Subject     string
	Body        string
	HTMLBody    string
	Attachments []Attachment
}

// Attachment rappresenta un allegato
type Attachment struct {
	Filename    string
	ContentType string
	Data        []byte
}

// PermessoEmailData dati per template email permesso
type PermessoEmailData struct {
	// Dati azienda
	RagioneSociale string
	IndirizzoAz    string
	TelefonoAz     string
	EmailAz        string
	
	// Dati nave
	NomeNave       string
	IMO            string
	NomeCompagnia  string
	
	// Dati porto
	NomePorto      string
	CittaPorto     string
	
	// Dati permesso
	TipoDurata     string
	DataInizio     string
	DataFine       string
	
	// Tecnici
	Tecnici        []TecnicoEmail
	
	// Veicolo
	Targa          string
	
	// Note
	DescrizioneIntervento string
	Note           string
	
	// Firma
	FirmaTesto     template.HTML
	FirmaPath      string
}

// TecnicoEmail dati tecnico per email
type TecnicoEmail struct {
	NomeCognome string
	Email       string
	Telefono    string
}

// GeneraCorpoEmailPermesso genera l'HTML dell'email per richiesta permesso
func GeneraCorpoEmailPermesso(data PermessoEmailData) (string, error) {
	tmpl := `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #003366; color: white; padding: 20px; text-align: center; }
        .content { padding: 20px; background: #f9f9f9; }
        .section { margin-bottom: 20px; }
        .section-title { font-weight: bold; color: #003366; border-bottom: 2px solid #003366; padding-bottom: 5px; margin-bottom: 10px; }
        .data-row { margin: 8px 0; }
        .label { font-weight: bold; display: inline-block; min-width: 150px; }
        table { width: 100%; border-collapse: collapse; margin-top: 10px; }
        th, td { border: 1px solid #ddd; padding: 10px; text-align: left; }
        th { background: #003366; color: white; }
        .footer { margin-top: 30px; padding-top: 20px; border-top: 2px solid #003366; }
        .firma { margin-top: 20px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h2>Richiesta Permesso Accesso Porto</h2>
        </div>
        <div class="content">
            <p>Gentili Signori,</p>
            <p>con la presente richiediamo il permesso di accesso al porto per i nostri tecnici per effettuare il seguente intervento sulla nave indicata.</p>
            
            <div class="section" style="background: #e8f4fc; border-left: 4px solid #0066cc; padding: 15px; margin: 15px 0;">
                <div class="section-title">DESCRIZIONE INTERVENTO</div>
                <p style="margin: 10px 0; white-space: pre-wrap;">{{.DescrizioneIntervento}}</p>
            </div>
            
            <div class="section">
                <div class="section-title">DATI NAVE</div>
                <div class="data-row"><span class="label">Nave:</span> {{.NomeNave}}</div>
                {{if .IMO}}<div class="data-row"><span class="label">IMO:</span> {{.IMO}}</div>{{end}}
                <div class="data-row"><span class="label">Compagnia:</span> {{.NomeCompagnia}}</div>
            </div>
            
            <div class="section">
                <div class="section-title">PORTO</div>
                <div class="data-row"><span class="label">Porto:</span> {{.NomePorto}}{{if .CittaPorto}} - {{.CittaPorto}}{{end}}</div>
            </div>
            
            <div class="section">
                <div class="section-title">PERIODO</div>
                <div class="data-row"><span class="label">Tipo:</span> {{.TipoDurata}}</div>
                <div class="data-row"><span class="label">Dal:</span> {{.DataInizio}}</div>
                {{if .DataFine}}<div class="data-row"><span class="label">Al:</span> {{.DataFine}}</div>{{end}}
            </div>
            
            <div class="section">
                <div class="section-title">TECNICI</div>
                <table>
                    <tr>
                        <th>Nome e Cognome</th>
                        <th>Email</th>
                        <th>Telefono</th>
                    </tr>
                    {{range .Tecnici}}
                    <tr>
                        <td>{{.NomeCognome}}</td>
                        <td>{{.Email}}</td>
                        <td>{{.Telefono}}</td>
                    </tr>
                    {{end}}
                </table>
            </div>
            
            {{if .Targa}}
            <div class="section">
                <div class="section-title">VEICOLO</div>
                <div class="data-row"><span class="label">Targa:</span> {{.Targa}}</div>
            </div>
            {{end}}
            
            {{if .Note}}
            <div class="section">
                <div class="section-title">NOTE</div>
                <p>{{.Note}}</p>
            </div>
            {{end}}
            
            <p>In attesa di Vostro cortese riscontro, porgiamo distinti saluti.</p>
            
            <div class="footer">
                <div class="firma">
                    {{if .FirmaTesto}}{{.FirmaTesto}}{{else}}
                    <strong>{{.RagioneSociale}}</strong><br>
                    {{if .IndirizzoAz}}{{.IndirizzoAz}}<br>{{end}}
                    {{if .TelefonoAz}}Tel: {{.TelefonoAz}}<br>{{end}}
                    {{if .EmailAz}}Email: {{.EmailAz}}{{end}}
                    {{end}}
                </div>
            </div>
        </div>
    </div>
</body>
</html>`

	t, err := template.New("email").Parse(tmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// GeneraOggettoEmailPermesso genera l'oggetto dell'email
func GeneraOggettoEmailPermesso(nomeNave, nomePorto string, dataInizio time.Time) string {
	return fmt.Sprintf("Richiesta Permesso Accesso Porto - %s - %s - %s", 
		nomeNave, nomePorto, dataInizio.Format("02/01/2006"))
}

// InviaEmail invia un'email tramite SMTP
func InviaEmail(config SMTPConfig, email EmailData) error {
	if config.Server == "" {
		return fmt.Errorf("server SMTP non configurato")
	}
	
	addr := fmt.Sprintf("%s:%d", config.Server, config.Port)
	auth := smtp.PlainAuth("", config.User, config.Password, config.Server)

	// Costruisci il messaggio
	var msg bytes.Buffer
	boundary := "----=_Part_" + fmt.Sprintf("%d", time.Now().UnixNano())

	// Headers
	msg.WriteString(fmt.Sprintf("From: %s <%s>\r\n", config.FromName, config.FromAddr))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(email.To, ", ")))
	if len(email.Cc) > 0 {
		msg.WriteString(fmt.Sprintf("Cc: %s\r\n", strings.Join(email.Cc, ", ")))
	}
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", email.Subject))
	msg.WriteString("MIME-Version: 1.0\r\n")

	if len(email.Attachments) > 0 {
		msg.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=\"%s\"\r\n", boundary))
		msg.WriteString("\r\n")
		
		// HTML body
		msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
		msg.WriteString("\r\n")
		msg.WriteString(email.HTMLBody)
		msg.WriteString("\r\n")
		
		// Attachments
		for _, att := range email.Attachments {
			msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
			msg.WriteString(fmt.Sprintf("Content-Type: %s; name=\"%s\"\r\n", att.ContentType, att.Filename))
			msg.WriteString("Content-Transfer-Encoding: base64\r\n")
			msg.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=\"%s\"\r\n", att.Filename))
			msg.WriteString("\r\n")
			msg.WriteString(base64.StdEncoding.EncodeToString(att.Data))
			msg.WriteString("\r\n")
		}
		
		msg.WriteString(fmt.Sprintf("--%s--\r\n", boundary))
	} else {
		msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
		msg.WriteString("\r\n")
		msg.WriteString(email.HTMLBody)
	}

	// Tutti i destinatari
	recipients := append(email.To, email.Cc...)
	
	return smtp.SendMail(addr, auth, config.FromAddr, recipients, msg.Bytes())
}

// CaricaAllegato carica un file come allegato
func CaricaAllegato(filePath string) (*Attachment, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	contentType := "application/octet-stream"
	switch ext {
	case ".pdf":
		contentType = "application/pdf"
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	case ".png":
		contentType = "image/png"
	case ".doc":
		contentType = "application/msword"
	case ".docx":
		contentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	}

	return &Attachment{
		Filename:    filepath.Base(filePath),
		ContentType: contentType,
		Data:        data,
	}, nil
}

// ConfigDaImpostazioni crea SMTPConfig dalle impostazioni azienda
func ConfigDaImpostazioni(imp *models.ImpostazioniAzienda) SMTPConfig {
	port := imp.SMTPPort
	if port == 0 {
		port = 587
	}
	
	fromName := imp.SMTPFromName
	if fromName == "" {
		fromName = imp.RagioneSociale
	}
	
	return SMTPConfig{
		Server:   imp.SMTPServer,
		Port:     port,
		User:     imp.SMTPUser,
		Password: imp.SMTPPassword,
		FromName: fromName,
		FromAddr: imp.Email,
	}
}
