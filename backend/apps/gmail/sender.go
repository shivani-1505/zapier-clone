package gmail

import (
	"fmt"
	"log"
	"net/smtp"
	"regexp"
	"strings"
	"time"
)

// Define a credentials type locally to avoid importing from auth
type Credentials struct {
	GmailAccount     string
	GmailAppPassword string
}

// Define a function variable for getting credentials
var GetCredentials func(userID string) (Credentials, error)

// SendEmail uses Gmail SMTP to send an email for a specific user
func SendEmail(userID, to, subject, messageText string) error {
	emailID := fmt.Sprintf("email-%d", time.Now().UnixNano())
	log.Printf("[%s] 📨 CRITICAL: Starting email send process for userID: %s", emailID, userID)
	// Extensive input validation
	if userID == "" {
		log.Printf("[%s] ❌ CRITICAL: UserID is empty", emailID)
		return fmt.Errorf("userID cannot be empty")
	}
	if to == "" {
		log.Printf("[%s] ❌ CRITICAL: No recipients specified", emailID)
		return fmt.Errorf("no recipients specified")
	}
	if subject == "" {
		subject = "No Subject"
	}
	if messageText == "" {
		log.Printf("[%s] ⚠️ WARNING: Empty message body", emailID)
		messageText = "Empty message body"
	}
	log.Printf("[%s] 📧 Email Details:", emailID)
	log.Printf("[%s] To: %s", emailID, to)
	log.Printf("[%s] Subject: %s", emailID, subject)
	log.Printf("[%s] Message Length: %d", emailID, len(messageText))
	// Validate recipient email format with stricter checks
	recipients := strings.FieldsFunc(to, func(r rune) bool {
		return r == ',' || r == ';' || r == ' '
	})
	log.Printf("[%s] 📧 Parsed %d potential recipients", emailID, len(recipients))
	validRecipients := make([]string, 0, len(recipients))
	for i, recipient := range recipients {
		recipient = strings.TrimSpace(recipient)
		if recipient == "" {
			log.Printf("[%s] ⚠️ Empty recipient found at position %d, skipping", emailID, i)
			continue
		}
		// More robust email validation
		emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
		if !emailRegex.MatchString(recipient) {
			log.Printf("[%s] ⚠️ Invalid email format: %s", emailID, recipient)
			continue
		}
		validRecipients = append(validRecipients, recipient)
	}
	// Check if we have any valid recipients
	if len(validRecipients) == 0 {
		log.Printf("[%s] ❌ CRITICAL: No valid recipients found after filtering", emailID)
		return fmt.Errorf("no valid email recipients found")
	}
	to = strings.Join(validRecipients, ",")
	log.Printf("[%s] 📧 Final validated recipient list: %s", emailID, to)

	// Get the SMTP credentials using the function variable
	if GetCredentials == nil {
		return fmt.Errorf("credentials manager not initialized")
	}

	creds, err := GetCredentials(userID)
	if err != nil {
		log.Printf("[%s] ❌ CRITICAL: SMTP credentials retrieval error: %+v", emailID, err)
		return fmt.Errorf("failed to retrieve SMTP credentials: %v", err)
	}

	log.Printf("[%s] 📧 Using sender email: %s", emailID, creds.GmailAccount)
	// SMTP server settings for Gmail
	smtpHost := "smtp.gmail.com"
	smtpPort := "587"
	from := creds.GmailAccount
	log.Printf("[%s] 📧 Using sender email: %s", emailID, from)
	// Construct email message
	log.Printf("[%s] 🔄 Constructing email message", emailID)
	// Format email headers and body
	message := []byte(fmt.Sprintf(
		"From: %s\r\n"+
			"To: %s\r\n"+
			"Subject: %s\r\n"+
			"MIME-Version: 1.0\r\n"+
			"Content-Type: text/plain; charset=\"UTF-8\"\r\n\r\n%s",
		from, to, subject, messageText,
	))
	// Send the email with comprehensive retry and error handling
	log.Printf("[%s] 📤 Sending email via Gmail SMTP...", emailID)
	maxRetries := 3
	var lastError error
	smtpAuth := smtp.PlainAuth("", from, creds.GmailAppPassword, smtpHost)
	addr := fmt.Sprintf("%s:%s", smtpHost, smtpPort)
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			log.Printf("[%s] 🔄 Retry attempt %d of %d", emailID, attempt+1, maxRetries)
			time.Sleep(time.Duration(attempt*2) * time.Second) // Exponential backoff
		}
		err := smtp.SendMail(addr, smtpAuth, from, validRecipients, message)
		if err == nil {
			log.Printf("[%s] ✅ Email sent successfully on attempt %d", emailID, attempt+1)
			return nil
		}
		log.Printf("[%s] ⚠️ Send Attempt %d Failed: %v", emailID, attempt+1, err)
		lastError = err
		// Check for authentication errors
		if strings.Contains(err.Error(), "authentication failed") ||
			strings.Contains(err.Error(), "auth") {
			log.Printf("[%s] 🚨 AUTHENTICATION ERROR: Check Gmail account and app password", emailID)
			return fmt.Errorf("gmail authentication failed: %v", err)
		}
		// For certain errors, don't retry
		if strings.Contains(err.Error(), "connection refused") ||
			strings.Contains(err.Error(), "no such host") {
			log.Printf("[%s] ❌ CRITICAL: SMTP connection error: %v", emailID, err)
			break
		}
	}
	// Final error reporting
	log.Printf("[%s] ❌ CRITICAL: Failed to send email after %d attempts", emailID, maxRetries)
	if lastError != nil {
		log.Printf("[%s] Final Error: %+v", emailID, lastError)
	}
	return fmt.Errorf("failed to send email after %d attempts: %v", maxRetries, lastError)
}

// SendEmailWithFallback uses hardcoded Gmail credentials to send an email
func SendEmailWithFallback(to, subject, messageText string) error {
	emailID := fmt.Sprintf("email-%d", time.Now().UnixNano())
	log.Printf("[%s] 📨 Starting email send process using fallback credentials", emailID)
	// Hardcoded fallback credentials
	from := "connectify.workflow@gmail.com"
	password := "dvhv tmod qdzu jyrj"
	// Input validation
	if to == "" {
		log.Printf("[%s] ❌ No recipients specified", emailID)
		return fmt.Errorf("no recipients specified")
	}
	if subject == "" {
		subject = "No Subject"
	}
	if messageText == "" {
		log.Printf("[%s] ⚠️ WARNING: Empty message body", emailID)
		messageText = "Empty message body"
	}
	log.Printf("[%s] 📧 Email Details:", emailID)
	log.Printf("[%s] To: %s", emailID, to)
	log.Printf("[%s] Subject: %s", emailID, subject)
	log.Printf("[%s] Message Length: %d", emailID, len(messageText))
	// Validate recipient email format with stricter checks
	recipients := strings.FieldsFunc(to, func(r rune) bool {
		return r == ',' || r == ';' || r == ' '
	})
	log.Printf("[%s] 📧 Parsed %d potential recipients", emailID, len(recipients))
	validRecipients := make([]string, 0, len(recipients))
	for i, recipient := range recipients {
		recipient = strings.TrimSpace(recipient)
		if recipient == "" {
			log.Printf("[%s] ⚠️ Empty recipient found at position %d, skipping", emailID, i)
			continue
		}
		// More robust email validation
		emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
		if !emailRegex.MatchString(recipient) {
			log.Printf("[%s] ⚠️ Invalid email format: %s", emailID, recipient)
			continue
		}
		validRecipients = append(validRecipients, recipient)
	}
	// Check if we have any valid recipients
	if len(validRecipients) == 0 {
		log.Printf("[%s] ❌ No valid recipients found after filtering", emailID)
		return fmt.Errorf("no valid email recipients found")
	}
	to = strings.Join(validRecipients, ",")
	log.Printf("[%s] 📧 Final validated recipient list: %s", emailID, to)
	// SMTP server settings for Gmail
	smtpHost := "smtp.gmail.com"
	smtpPort := "587"
	log.Printf("[%s] 📧 Using sender email: %s", emailID, from)
	// Construct email message
	log.Printf("[%s] 🔄 Constructing email message", emailID)
	// Format email headers and body
	message := []byte(fmt.Sprintf(
		"From: %s\r\n"+
			"To: %s\r\n"+
			"Subject: %s\r\n"+
			"MIME-Version: 1.0\r\n"+
			"Content-Type: text/plain; charset=\"UTF-8\"\r\n\r\n%s",
		from, to, subject, messageText,
	))
	// Send the email with comprehensive retry and error handling
	log.Printf("[%s] 📤 Sending email via Gmail SMTP...", emailID)
	maxRetries := 3
	var lastError error
	smtpAuth := smtp.PlainAuth("", from, password, smtpHost)
	addr := fmt.Sprintf("%s:%s", smtpHost, smtpPort)
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			log.Printf("[%s] 🔄 Retry attempt %d of %d", emailID, attempt+1, maxRetries)
			time.Sleep(time.Duration(attempt*2) * time.Second) // Exponential backoff
		}
		err := smtp.SendMail(addr, smtpAuth, from, validRecipients, message)
		if err == nil {
			log.Printf("[%s] ✅ Email sent successfully on attempt %d", emailID, attempt+1)
			return nil
		}
		log.Printf("[%s] ⚠️ Send Attempt %d Failed: %v", emailID, attempt+1, err)
		lastError = err
		// Check for authentication errors
		if strings.Contains(err.Error(), "authentication failed") ||
			strings.Contains(err.Error(), "auth") {
			log.Printf("[%s] 🚨 AUTHENTICATION ERROR: Check Gmail account and app password", emailID)
			return fmt.Errorf("gmail authentication failed: %v", err)
		}
		// For certain errors, don't retry
		if strings.Contains(err.Error(), "connection refused") ||
			strings.Contains(err.Error(), "no such host") {
			log.Printf("[%s] ❌ CRITICAL: SMTP connection error: %v", emailID, err)
			break
		}
	}
	// Final error reporting
	log.Printf("[%s] ❌ Failed to send email after %d attempts", emailID, maxRetries)
	if lastError != nil {
		log.Printf("[%s] Final Error: %+v", emailID, lastError)
	}
	return fmt.Errorf("failed to send email after %d attempts: %v", maxRetries, lastError)
}
