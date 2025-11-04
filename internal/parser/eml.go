package parser

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"os"
	"strings"
	"time"

	"github.com/emersion/go-message/charset"
	"github.com/emersion/go-message/mail"
	"golang.org/x/text/encoding/charmap"
)

func init() {
	// Register additional charsets that are commonly used in emails
	charset.RegisterEncoding("windows-1252", charmap.Windows1252)
	charset.RegisterEncoding("iso-8859-1", charmap.ISO8859_1)
	charset.RegisterEncoding("iso-8859-15", charmap.ISO8859_15)
}

// ParseEMLFile parses an .eml file and returns a ParsedEmail
func ParseEMLFile(filePath string) (*ParsedEmail, error) {
	// Open the file
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	// Parse the email
	return ParseEML(f)
}

// ParseEML parses an email from a reader
func ParseEML(r io.Reader) (*ParsedEmail, error) {
	// Read the entire message first to capture raw headers
	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, r); err != nil {
		return nil, fmt.Errorf("failed to read email: %w", err)
	}

	// Parse the message
	mr, err := mail.CreateReader(bytes.NewReader(buf.Bytes()))
	if err != nil {
		return nil, fmt.Errorf("failed to create mail reader: %w", err)
	}

	parsed := &ParsedEmail{}

	// Extract raw headers
	parsed.RawHeaders = extractRawHeaders(buf.String())

	// Parse headers
	header := mr.Header

	// Message-ID
	if msgID := header.Get("Message-Id"); msgID != "" {
		parsed.MessageID = msgID
	}

	// In-Reply-To (for threading)
	if inReplyTo := header.Get("In-Reply-To"); inReplyTo != "" {
		parsed.InReplyTo = strings.TrimSpace(inReplyTo)
	}

	// References (for threading)
	if references := header.Get("References"); references != "" {
		// References can be space-separated Message-IDs
		parsed.References = parseMessageIDList(references)
	}

	// Subject - decode MIME words
	parsed.Subject = decodeMIMEWord(header.Get("Subject"))

	// From
	if fromAddrs, err := header.AddressList("From"); err == nil && len(fromAddrs) > 0 {
		parsed.Sender = fromAddrs[0].Address
		parsed.SenderName = fromAddrs[0].Name
	}

	// To
	if toAddrs, err := header.AddressList("To"); err == nil {
		for _, addr := range toAddrs {
			parsed.Recipients = append(parsed.Recipients, addr.Address)
		}
	}

	// CC
	if ccAddrs, err := header.AddressList("Cc"); err == nil {
		for _, addr := range ccAddrs {
			parsed.CC = append(parsed.CC, addr.Address)
		}
	}

	// BCC
	if bccAddrs, err := header.AddressList("Bcc"); err == nil {
		for _, addr := range bccAddrs {
			parsed.BCC = append(parsed.BCC, addr.Address)
		}
	}

	// Date
	if date, err := header.Date(); err == nil {
		parsed.Date = date
	} else {
		// Use current time as fallback
		parsed.Date = time.Now()
	}

	// Parse body and attachments
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read part: %w", err)
		}

		switch h := part.Header.(type) {
		case *mail.InlineHeader:
			// This is the message body
			contentType, _, _ := h.ContentType()
			body, err := io.ReadAll(part.Body)
			if err != nil {
				return nil, fmt.Errorf("failed to read body: %w", err)
			}

			if strings.HasPrefix(contentType, "text/plain") {
				// Keep text even if we already have it (multipart emails have both)
				if parsed.BodyText == "" {
					parsed.BodyText = string(body)
				}
			} else if strings.HasPrefix(contentType, "text/html") {
				// Always prefer HTML if available
				parsed.BodyHTML = string(body)
			}

		case *mail.AttachmentHeader:
			// This is an attachment
			filename, _ := h.Filename()
			contentType, _, _ := h.ContentType()

			data, err := io.ReadAll(part.Body)
			if err != nil {
				return nil, fmt.Errorf("failed to read attachment: %w", err)
			}

			parsed.Attachments = append(parsed.Attachments, ParsedAttachment{
				Filename:    filename,
				ContentType: contentType,
				Size:        int64(len(data)),
				Data:        data,
			})
		}
	}

	return parsed, nil
}

// extractRawHeaders extracts the raw header section from the email
func extractRawHeaders(emailContent string) string {
	// Headers end at the first blank line
	parts := strings.SplitN(emailContent, "\r\n\r\n", 2)
	if len(parts) < 2 {
		parts = strings.SplitN(emailContent, "\n\n", 2)
	}
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// decodeMIMEWord decodes MIME-encoded words (RFC 2047)
// Example: =?UTF-8?Q?Invitaci=C3=B3n?= -> Invitación
func decodeMIMEWord(s string) string {
	dec := new(mime.WordDecoder)
	decoded, err := dec.DecodeHeader(s)
	if err != nil {
		// If decoding fails, return original string
		return s
	}
	return decoded
}

// parseMessageIDList parses a space-separated list of Message-IDs
// Example: "<id1@example.com> <id2@example.com>" -> ["<id1@example.com>", "<id2@example.com>"]
func parseMessageIDList(s string) []string {
	var ids []string
	// Message-IDs are enclosed in angle brackets < >
	// Split by whitespace and filter out empty strings
	parts := strings.Fields(s)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			ids = append(ids, part)
		}
	}
	return ids
}
