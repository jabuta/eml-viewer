package parser

import "time"

// ParsedEmail represents a parsed email with all its components
type ParsedEmail struct {
	MessageID   string
	Subject     string
	Sender      string
	SenderName  string
	Recipients  []string
	CC          []string
	BCC         []string
	Date        time.Time
	BodyText    string
	BodyHTML    string
	Attachments []ParsedAttachment
	RawHeaders  string
}

// ParsedAttachment represents an email attachment
type ParsedAttachment struct {
	Filename    string
	ContentType string
	Size        int64
	Data        []byte
}
