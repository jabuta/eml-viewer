package db

const schema = `
-- Main emails table
CREATE TABLE IF NOT EXISTS emails (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    file_path TEXT UNIQUE NOT NULL,
    message_id TEXT,
    subject TEXT,
    sender TEXT NOT NULL,
    sender_name TEXT,
    recipients TEXT,
    cc TEXT,
    bcc TEXT,
    date DATETIME,
    body_text TEXT,
    body_html TEXT,
    has_attachments BOOLEAN DEFAULT 0,
    attachment_count INTEGER DEFAULT 0,
    raw_headers TEXT,
    file_size INTEGER,
    indexed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Full-text search virtual table
CREATE VIRTUAL TABLE IF NOT EXISTS emails_fts USING fts5(
    subject,
    sender,
    sender_name,
    recipients,
    body_text,
    content='emails',
    content_rowid='id'
);

-- Triggers to keep FTS in sync
CREATE TRIGGER IF NOT EXISTS emails_ai AFTER INSERT ON emails BEGIN
    INSERT INTO emails_fts(rowid, subject, sender, sender_name, recipients, body_text)
    VALUES (new.id, new.subject, new.sender, new.sender_name, new.recipients, new.body_text);
END;

CREATE TRIGGER IF NOT EXISTS emails_ad AFTER DELETE ON emails BEGIN
    DELETE FROM emails_fts WHERE rowid = old.id;
END;

CREATE TRIGGER IF NOT EXISTS emails_au AFTER UPDATE ON emails BEGIN
    UPDATE emails_fts
    SET subject = new.subject,
        sender = new.sender,
        sender_name = new.sender_name,
        recipients = new.recipients,
        body_text = new.body_text
    WHERE rowid = new.id;
END;

-- Attachments table
CREATE TABLE IF NOT EXISTS attachments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email_id INTEGER NOT NULL,
    filename TEXT NOT NULL,
    content_type TEXT,
    size INTEGER,
    data BLOB,
    FOREIGN KEY(email_id) REFERENCES emails(id) ON DELETE CASCADE
);

-- Settings table (for storing email folder path, preferences)
CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_emails_date ON emails(date DESC);
CREATE INDEX IF NOT EXISTS idx_emails_sender ON emails(sender);
CREATE INDEX IF NOT EXISTS idx_emails_file_path ON emails(file_path);
CREATE INDEX IF NOT EXISTS idx_attachments_email_id ON attachments(email_id);
`
