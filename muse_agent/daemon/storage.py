from __future__ import annotations

import json
import sqlite3
import time
from dataclasses import dataclass
from enum import Enum
from pathlib import Path
from typing import Any, Dict, List, Optional


class TaskPriority(str, Enum):
    LOW = "low"
    NORMAL = "normal"
    HIGH = "high"
    CRITICAL = "critical"


class TaskStatus(str, Enum):
    PENDING = "pending"
    RUNNING = "running"
    COMPLETED = "completed"
    FAILED = "failed"
    CANCELLED = "cancelled"


@dataclass
class QueuedTask:
    id: int
    goal: str
    priority: TaskPriority
    status: TaskStatus
    max_steps: int
    created_at: float
    started_at: Optional[float] = None
    completed_at: Optional[float] = None
    result_summary: Optional[str] = None
    error: Optional[str] = None
    is_recurring: bool = False
    schedule_id: Optional[int] = None


@dataclass
class ScheduledJob:
    id: int
    name: str
    goal: str
    cron_expr: Optional[str]
    interval_seconds: Optional[int]
    last_run: Optional[float]
    next_run: float
    enabled: bool
    priority: TaskPriority = TaskPriority.NORMAL


class DaemonStorage:
    """
    Persistent SQLite storage engine for the 24/7 Autonomous Daemon.
    Tracks persistent task queues, recurring cron schedules, telemetry heartbeats,
    and cross-session learned desktop memories.
    """

    def __init__(self, db_path: Path = Path("data/agent_state.db")):
        self.db_path = db_path
        self.db_path.parent.mkdir(parents=True, exist_ok=True)
        self._init_db()

    def _get_conn(self) -> sqlite3.Connection:
        conn = sqlite3.connect(str(self.db_path), timeout=15.0)
        conn.row_factory = sqlite3.Row
        return conn

    def _init_db(self) -> None:
        with self._get_conn() as conn:
            conn.executescript("""
            CREATE TABLE IF NOT EXISTS tasks (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                goal TEXT NOT NULL,
                priority TEXT DEFAULT 'normal',
                status TEXT DEFAULT 'pending',
                max_steps INTEGER DEFAULT 25,
                created_at REAL NOT NULL,
                started_at REAL,
                completed_at REAL,
                result_summary TEXT,
                error TEXT,
                is_recurring INTEGER DEFAULT 0,
                schedule_id INTEGER
            );

            CREATE TABLE IF NOT EXISTS schedules (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                name TEXT NOT NULL,
                goal TEXT NOT NULL,
                cron_expr TEXT,
                interval_seconds INTEGER,
                last_run REAL,
                next_run REAL NOT NULL,
                enabled INTEGER DEFAULT 1,
                priority TEXT DEFAULT 'normal'
            );

            CREATE TABLE IF NOT EXISTS system_heartbeat (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                timestamp REAL NOT NULL,
                uptime_seconds REAL NOT NULL,
                cpu_percent REAL,
                memory_percent REAL,
                battery_percent REAL,
                is_charging INTEGER,
                user_idle_seconds REAL,
                daemon_status TEXT
            );

            CREATE TABLE IF NOT EXISTS learned_memories (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                category TEXT NOT NULL,
                key TEXT NOT NULL UNIQUE,
                value_json TEXT NOT NULL,
                confidence REAL DEFAULT 1.0,
                updated_at REAL NOT NULL
            );

            CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
            CREATE INDEX IF NOT EXISTS idx_tasks_priority ON tasks(priority);
            CREATE INDEX IF NOT EXISTS idx_schedules_next_run ON schedules(next_run, enabled);
            """)

    def add_task(
        self,
        goal: str,
        priority: TaskPriority = TaskPriority.NORMAL,
        max_steps: int = 25,
        schedule_id: Optional[int] = None
    ) -> int:
        now = time.time()
        with self._get_conn() as conn:
            cur = conn.execute(
                """
                INSERT INTO tasks (goal, priority, status, max_steps, created_at, is_recurring, schedule_id)
                VALUES (?, ?, ?, ?, ?, ?, ?)
                """,
                (goal, priority.value, TaskStatus.PENDING.value, max_steps, now, 1 if schedule_id else 0, schedule_id)
            )
            return cur.lastrowid

    def fetch_next_pending_task(self) -> Optional[QueuedTask]:
        """Fetch highest-priority pending task based on priority order: critical > high > normal > low."""
        priority_order = "CASE priority WHEN 'critical' THEN 1 WHEN 'high' THEN 2 WHEN 'normal' THEN 3 ELSE 4 END"
        with self._get_conn() as conn:
            row = conn.execute(
                f"""
                SELECT * FROM tasks
                WHERE status = 'pending'
                ORDER BY {priority_order}, created_at ASC
                LIMIT 1
                """
            ).fetchone()
            if row:
                return QueuedTask(
                    id=row["id"],
                    goal=row["goal"],
                    priority=TaskPriority(row["priority"]),
                    status=TaskStatus(row["status"]),
                    max_steps=row["max_steps"],
                    created_at=row["created_at"],
                    started_at=row["started_at"],
                    completed_at=row["completed_at"],
                    result_summary=row["result_summary"],
                    error=row["error"],
                    is_recurring=bool(row["is_recurring"]),
                    schedule_id=row["schedule_id"]
                )
            return None

    def update_task_status(
        self,
        task_id: int,
        status: TaskStatus,
        result_summary: Optional[str] = None,
        error: Optional[str] = None
    ) -> None:
        now = time.time()
        with self._get_conn() as conn:
            if status == TaskStatus.RUNNING:
                conn.execute("UPDATE tasks SET status = ?, started_at = ? WHERE id = ?", (status.value, now, task_id))
            elif status in (TaskStatus.COMPLETED, TaskStatus.FAILED, TaskStatus.CANCELLED):
                conn.execute(
                    """
                    UPDATE tasks
                    SET status = ?, completed_at = ?, result_summary = ?, error = ?
                    WHERE id = ?
                    """,
                    (status.value, now, result_summary, error, task_id)
                )

    def list_tasks(self, limit: int = 50, status: Optional[str] = None) -> List[Dict[str, Any]]:
        with self._get_conn() as conn:
            if status:
                rows = conn.execute(
                    "SELECT * FROM tasks WHERE status = ? ORDER BY id DESC LIMIT ?", (status, limit)
                ).fetchall()
            else:
                rows = conn.execute("SELECT * FROM tasks ORDER BY id DESC LIMIT ?", (limit,)).fetchall()
            return [dict(r) for r in rows]

    def add_schedule(
        self,
        name: str,
        goal: str,
        cron_expr: Optional[str] = None,
        interval_seconds: Optional[int] = None,
        priority: TaskPriority = TaskPriority.NORMAL
    ) -> int:
        now = time.time()
        next_run = now + (interval_seconds or 3600)
        with self._get_conn() as conn:
            cur = conn.execute(
                """
                INSERT INTO schedules (name, goal, cron_expr, interval_seconds, next_run, enabled, priority)
                VALUES (?, ?, ?, ?, ?, 1, ?)
                """,
                (name, goal, cron_expr, interval_seconds, next_run, priority.value)
            )
            return cur.lastrowid

    def list_schedules(self) -> List[Dict[str, Any]]:
        with self._get_conn() as conn:
            rows = conn.execute("SELECT * FROM schedules ORDER BY id ASC").fetchall()
            return [dict(r) for r in rows]

    def update_schedule_run(self, schedule_id: int, next_run: float) -> None:
        now = time.time()
        with self._get_conn() as conn:
            conn.execute(
                "UPDATE schedules SET last_run = ?, next_run = ? WHERE id = ?",
                (now, next_run, schedule_id)
            )

    def record_heartbeat(
        self,
        uptime_seconds: float,
        cpu_percent: float,
        memory_percent: float,
        battery_percent: float,
        is_charging: bool,
        user_idle_seconds: float,
        daemon_status: str
    ) -> None:
        now = time.time()
        with self._get_conn() as conn:
            conn.execute(
                """
                INSERT INTO system_heartbeat (timestamp, uptime_seconds, cpu_percent, memory_percent, battery_percent, is_charging, user_idle_seconds, daemon_status)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (now, uptime_seconds, cpu_percent, memory_percent, battery_percent, 1 if is_charging else 0, user_idle_seconds, daemon_status)
            )
            # Prune old heartbeats to keep db compact (keep last 10,000 records)
            conn.execute("DELETE FROM system_heartbeat WHERE id < (SELECT MAX(id) - 10000 FROM system_heartbeat)")

    def get_latest_heartbeat(self) -> Optional[Dict[str, Any]]:
        with self._get_conn() as conn:
            row = conn.execute("SELECT * FROM system_heartbeat ORDER BY id DESC LIMIT 1").fetchone()
            return dict(row) if row else None

