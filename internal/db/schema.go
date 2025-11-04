package db

// Optimized schema that stores only metadata + search index
// Full content (body_html, raw_headers, attachment data) is parsed from .eml files on-demand
const schema = `
-- Main emails table (metadata only)
CREATE TABLE IF NOT EXISTS emails (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    file_path TEXT UNIQUE NOT NULL,
    message_id TEXT,
    in_reply_to TEXT,        -- Message-ID of parent email (for threading)
    thread_references TEXT,  -- Comma-separated Message-IDs (conversation ancestry)
    subject TEXT,
    sender TEXT NOT NULL,
    sender_name TEXT,
    recipients TEXT,
    date DATETIME,
    body_text_preview TEXT,  -- First 10KB for FTS5 search only
    has_attachments BOOLEAN DEFAULT 0,
    attachment_count INTEGER DEFAULT 0,
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
    body_text_preview,
    content='emails',
    content_rowid='id'
);

-- Triggers to keep FTS in sync
CREATE TRIGGER IF NOT EXISTS emails_ai AFTER INSERT ON emails BEGIN
    INSERT INTO emails_fts(rowid, subject, sender, sender_name, recipients, body_text_preview)
    VALUES (new.id, new.subject, new.sender, new.sender_name, new.recipients, new.body_text_preview);
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
        body_text_preview = new.body_text_preview
    WHERE rowid = new.id;
END;

-- Attachments table (metadata only, no BLOB data)
CREATE TABLE IF NOT EXISTS attachments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email_id INTEGER NOT NULL,
    filename TEXT NOT NULL,
    content_type TEXT,
    size INTEGER,
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
CREATE INDEX IF NOT EXISTS idx_emails_sender_date ON emails(sender, date DESC); -- Composite index for grouped sender queries
CREATE INDEX IF NOT EXISTS idx_emails_file_path ON emails(file_path);
CREATE INDEX IF NOT EXISTS idx_emails_message_id ON emails(message_id);
CREATE INDEX IF NOT EXISTS idx_emails_in_reply_to ON emails(in_reply_to);
CREATE INDEX IF NOT EXISTS idx_attachments_email_id ON attachments(email_id);
`

// Migration schema for upgrading existing databases
const migrationSchema = `
-- Migration: Remove duplicate data columns
-- These columns store data that can be parsed from .eml files on-demand

-- Step 1: Create new optimized emails table
CREATE TABLE IF NOT EXISTS emails_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    file_path TEXT UNIQUE NOT NULL,
    message_id TEXT,
    subject TEXT,
    sender TEXT NOT NULL,
    sender_name TEXT,
    recipients TEXT,
    date DATETIME,
    body_text_preview TEXT,
    has_attachments BOOLEAN DEFAULT 0,
    attachment_count INTEGER DEFAULT 0,
    file_size INTEGER,
    indexed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Step 2: Copy data (truncating body_text to 10KB)
INSERT INTO emails_new
SELECT
    id, file_path, message_id, subject, sender, sender_name,
    recipients, date,
    SUBSTR(body_text, 1, 10240) as body_text_preview,  -- First 10KB
    has_attachments, attachment_count, file_size,
    indexed_at, updated_at
FROM emails;

-- Step 3: Drop old table and rename
DROP TABLE emails;
ALTER TABLE emails_new RENAME TO emails;

-- Step 4: Recreate indexes
CREATE INDEX IF NOT EXISTS idx_emails_date ON emails(date DESC);
CREATE INDEX IF NOT EXISTS idx_emails_sender ON emails(sender);
CREATE INDEX IF NOT EXISTS idx_emails_file_path ON emails(file_path);

-- Step 5: Rebuild FTS5 index
DROP TABLE IF EXISTS emails_fts;
CREATE VIRTUAL TABLE emails_fts USING fts5(
    subject,
    sender,
    sender_name,
    recipients,
    body_text_preview,
    content='emails',
    content_rowid='id'
);

INSERT INTO emails_fts(rowid, subject, sender, sender_name, recipients, body_text_preview)
SELECT id, subject, sender, sender_name, recipients, body_text_preview FROM emails;

-- Step 6: Recreate triggers
DROP TRIGGER IF EXISTS emails_ai;
DROP TRIGGER IF EXISTS emails_ad;
DROP TRIGGER IF EXISTS emails_au;

CREATE TRIGGER emails_ai AFTER INSERT ON emails BEGIN
    INSERT INTO emails_fts(rowid, subject, sender, sender_name, recipients, body_text_preview)
    VALUES (new.id, new.subject, new.sender, new.sender_name, new.recipients, new.body_text_preview);
END;

CREATE TRIGGER emails_ad AFTER DELETE ON emails BEGIN
    DELETE FROM emails_fts WHERE rowid = old.id;
END;

CREATE TRIGGER emails_au AFTER UPDATE ON emails BEGIN
    UPDATE emails_fts
    SET subject = new.subject,
        sender = new.sender,
        sender_name = new.sender_name,
        recipients = new.recipients,
        body_text_preview = new.body_text_preview
    WHERE rowid = new.id;
END;

-- Step 7: Optimize attachments table (remove BLOB data)
CREATE TABLE IF NOT EXISTS attachments_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email_id INTEGER NOT NULL,
    filename TEXT NOT NULL,
    content_type TEXT,
    size INTEGER,
    FOREIGN KEY(email_id) REFERENCES emails(id) ON DELETE CASCADE
);

INSERT INTO attachments_new
SELECT id, email_id, filename, content_type, size
FROM attachments;

DROP TABLE attachments;
ALTER TABLE attachments_new RENAME TO attachments;

CREATE INDEX IF NOT EXISTS idx_attachments_email_id ON attachments(email_id);

-- Step 8: VACUUM to reclaim space
VACUUM;
`
