package persistence

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct {
	conn *sql.DB
	mu   sync.RWMutex
}

func NewDB(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	conn.SetMaxOpenConns(1) // SQLite single-writer safety

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}
	return db, nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS tasks (
		id TEXT PRIMARY KEY,
		goal TEXT NOT NULL,
		status TEXT NOT NULL,
		model TEXT NOT NULL,
		max_steps INTEGER NOT NULL,
		current_step INTEGER DEFAULT 0,
		error TEXT,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS task_steps (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id TEXT NOT NULL,
		step_number INTEGER NOT NULL,
		action_type TEXT NOT NULL,
		command TEXT,
		thought TEXT,
		result TEXT,
		success BOOLEAN NOT NULL,
		created_at DATETIME NOT NULL,
		FOREIGN KEY(task_id) REFERENCES tasks(id)
	);

	CREATE TABLE IF NOT EXISTS execution_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id TEXT NOT NULL,
		event_type TEXT NOT NULL,
		payload TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		FOREIGN KEY(task_id) REFERENCES tasks(id)
	);

	CREATE TABLE IF NOT EXISTS approval_requests (
		id TEXT PRIMARY KEY,
		task_id TEXT NOT NULL,
		command TEXT NOT NULL,
		reason TEXT NOT NULL,
		severity TEXT NOT NULL,
		status TEXT NOT NULL, -- pending, approved, rejected
		created_at DATETIME NOT NULL,
		resolved_at DATETIME
	);

	CREATE TABLE IF NOT EXISTS memory_entries (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		key TEXT NOT NULL UNIQUE,
		value TEXT NOT NULL,
		category TEXT NOT NULL,
		created_at DATETIME NOT NULL
	);
	`
	_, err := db.conn.Exec(schema)
	return err
}

func (db *DB) CreateTask(id, goal, model string, maxSteps int) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	now := time.Now().UTC()
	query := `INSERT INTO tasks (id, goal, status, model, max_steps, current_step, created_at, updated_at) 
	          VALUES (?, ?, 'QUEUED', ?, ?, 0, ?, ?)`
	_, err := db.conn.Exec(query, id, goal, model, maxSteps, now, now)
	return err
}

func (db *DB) UpdateTaskStatus(id, status, errStr string, step int) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	now := time.Now().UTC()
	query := `UPDATE tasks SET status = ?, error = ?, current_step = ?, updated_at = ? WHERE id = ?`
	_, err := db.conn.Exec(query, status, errStr, step, now, id)
	return err
}

func (db *DB) RecordStep(taskID string, stepNum int, actionType, command, thought, result string, success bool) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	query := `INSERT INTO task_steps (task_id, step_number, action_type, command, thought, result, success, created_at)
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := db.conn.Exec(query, taskID, stepNum, actionType, command, thought, result, success, time.Now().UTC())
	return err
}

func (db *DB) RecordEvent(taskID, eventType, payload string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	query := `INSERT INTO execution_events (task_id, event_type, payload, created_at) VALUES (?, ?, ?, ?)`
	_, err := db.conn.Exec(query, taskID, eventType, payload, time.Now().UTC())
	return err
}

func (db *DB) CreateApproval(id, taskID, command, reason, severity string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	query := `INSERT INTO approval_requests (id, task_id, command, reason, severity, status, created_at) 
	          VALUES (?, ?, ?, ?, ?, 'pending', ?)`
	_, err := db.conn.Exec(query, id, taskID, command, reason, severity, time.Now().UTC())
	return err
}

func (db *DB) ResolveApproval(id string, approved bool) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	status := "approved"
	if !approved {
		status = "rejected"
	}
	query := `UPDATE approval_requests SET status = ?, resolved_at = ? WHERE id = ?`
	_, err := db.conn.Exec(query, status, time.Now().UTC(), id)
	return err
}

