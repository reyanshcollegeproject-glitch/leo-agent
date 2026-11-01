from __future__ import annotations

import logging
import time
from datetime import datetime
from typing import List, Optional
from croniter import croniter

from muse_agent.daemon.storage import DaemonStorage, ScheduledJob, TaskPriority

logger = logging.getLogger("muse_agent.daemon.scheduler")


class AutonomousScheduler:
    """
    Cron and interval-based recurring job scheduler for 24/7 autonomous operations.
    Evaluates active schedules and enqueues tasks into DaemonStorage.
    """

    def __init__(self, storage: DaemonStorage):
        self.storage = storage

    def compute_next_run(self, cron_expr: Optional[str], interval_seconds: Optional[int], base_time: float) -> float:
        """Compute the next unix timestamp for a recurring job."""
        if interval_seconds:
            return base_time + interval_seconds

        if cron_expr:
            try:
                base_dt = datetime.fromtimestamp(base_time)
                itr = croniter(cron_expr, base_dt)
                next_dt = itr.get_next(datetime)
                return next_dt.timestamp()
            except Exception as e:
                logger.error(f"Invalid cron expression '{cron_expr}': {e}")
                return base_time + 3600  # fallback 1h

        return base_time + 86400  # Default 24h

    def poll_and_enqueue_due_jobs(self) -> List[int]:
        """
        Check all enabled schedules. If due, enqueue task and update next_run.
        Returns list of enqueued task IDs.
        """
        now = time.time()
        enqueued_ids: List[int] = []
        schedules = self.storage.list_schedules()

        for s in schedules:
            if not s["enabled"]:
                continue

            if now >= s["next_run"]:
                # Job is due! Enqueue into tasks table
                priority = TaskPriority(s.get("priority", "normal"))
                task_id = self.storage.add_task(
                    goal=s["goal"],
                    priority=priority,
                    schedule_id=s["id"]
                )
                enqueued_ids.append(task_id)

                # Compute next execution time
                next_run = self.compute_next_run(s["cron_expr"], s["interval_seconds"], now)
                self.storage.update_schedule_run(s["id"], next_run)
                logger.info(f"Enqueued scheduled job '{s['name']}' (Task #{task_id}). Next run: {datetime.fromtimestamp(next_run)}")

        return enqueued_ids

