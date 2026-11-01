from __future__ import annotations

import asyncio
import logging
import os
import signal
import sys
import time
from pathlib import Path
from typing import Optional

from muse_agent.config import settings
from muse_agent.core.agent import MuseAgent
from muse_agent.daemon.scheduler import AutonomousScheduler
from muse_agent.daemon.storage import DaemonStorage, TaskStatus, TaskPriority
from muse_agent.daemon.watchers import SystemWatcher
from muse_agent.models.meta_muse import MetaMuseModel

logger = logging.getLogger("muse_agent.daemon.service")


class AutonomousDaemon:
    """
    24/7 Production Autonomous OS Daemon.
    Perpetual background service that monitors laptop health, executes scheduled jobs,
    dispatches queued tasks, and protects user focus with intelligent idle-state arbitration.
    """

    def __init__(
        self,
        pid_file: Path = Path("data/daemon.pid"),
        log_file: Path = Path("logs/daemon.log")
    ):
        self.pid_file = pid_file
        self.log_file = log_file
        self.storage = DaemonStorage()
        self.watcher = SystemWatcher(idle_threshold_seconds=120.0)
        self.scheduler = AutonomousScheduler(self.storage)
        self.running = False
        self.start_time = time.time()
        self.current_agent: Optional[MuseAgent] = None

    def _setup_logging(self) -> None:
        self.log_file.parent.mkdir(parents=True, exist_ok=True)
        handler = logging.FileHandler(self.log_file)
        formatter = logging.Formatter("[%(asctime)s] [%(levelname)s] %(name)s: %(message)s")
        handler.setFormatter(formatter)
        logging.getLogger().addHandler(handler)
        logging.getLogger().setLevel(logging.INFO)

    def write_pid(self) -> None:
        self.pid_file.parent.mkdir(parents=True, exist_ok=True)
        with open(self.pid_file, "w") as f:
            f.write(str(os.getpid()))

    def remove_pid(self) -> None:
        if self.pid_file.exists():
            try:
                self.pid_file.unlink()
            except Exception:
                pass

    def stop(self) -> None:
        logger.info("Stopping 24/7 Autonomous Daemon...")
        self.running = False
        if self.current_agent:
            self.current_agent.stop()
        self.remove_pid()

    async def start(self) -> None:
        """Main 24/7 autonomous loop."""
        self._setup_logging()
        self.write_pid()
        self.running = True

        # Signal handlers
        def sig_handler(sig, frame):
            self.stop()
            sys.exit(0)

        signal.signal(signal.SIGINT, sig_handler)
        signal.signal(signal.SIGTERM, sig_handler)

        logger.info(f"🚀 Meta Muse 24/7 Autonomous Daemon started (PID {os.getpid()})")
        logger.info(f"Model: {settings.muse_model_name} | Endpoint: {settings.muse_endpoint}")

        # Seed initial default system schedules if empty
        self._seed_default_schedules()

        heartbeat_counter = 0

        while self.running:
            try:
                now = time.time()
                uptime = now - self.start_time

                # 1. System Telemetry & Heartbeat (every 10s)
                snapshot = self.watcher.capture_snapshot()
                if heartbeat_counter % 2 == 0:
                    self.storage.record_heartbeat(
                        uptime_seconds=uptime,
                        cpu_percent=snapshot.cpu_percent,
                        memory_percent=snapshot.memory_percent,
                        battery_percent=snapshot.battery_percent,
                        is_charging=snapshot.is_charging,
                        user_idle_seconds=snapshot.user_idle_seconds,
                        daemon_status="running"
                    )
                heartbeat_counter += 1

                # 2. Poll recurring scheduled jobs
                self.scheduler.poll_and_enqueue_due_jobs()

                # 3. Check for next queued task
                task = self.storage.fetch_next_pending_task()

                if task:
                    # User Activity Arbitration:
                    # If task requires screen automation and user is actively working, pause to avoid stealing cursor
                    if snapshot.is_user_active and task.priority != TaskPriority.CRITICAL:
                        logger.info(f"Deferring Task #{task.id} (user actively interacting with mouse/keyboard, idle: {snapshot.user_idle_seconds}s)")
                        await asyncio.sleep(5.0)
                        continue

                    # Execute task
                    logger.info(f"Starting execution of Task #{task.id} [Priority: {task.priority.value}]: \"{task.goal}\"")
                    self.storage.update_task_status(task.id, TaskStatus.RUNNING)

                    model = MetaMuseModel(
                        api_key=settings.muse_api_key,
                        model_name=settings.muse_model_name,
                        endpoint=settings.muse_endpoint
                    )
                    self.current_agent = MuseAgent(model=model, run_id=f"daemon_task_{task.id}")

                    exec_res = await self.current_agent.run(goal=task.goal, max_steps=task.max_steps)

                    if exec_res.get("success"):
                        self.storage.update_task_status(
                            task.id,
                            TaskStatus.COMPLETED,
                            result_summary=f"Completed in {exec_res.get('total_steps')} steps ({exec_res.get('elapsed_seconds')}s)"
                        )
                        logger.info(f"Task #{task.id} finished successfully!")
                    else:
                        self.storage.update_task_status(
                            task.id,
                            TaskStatus.FAILED,
                            error=exec_res.get("error", "Task aborted or limit reached")
                        )
                        logger.warning(f"Task #{task.id} failed: {exec_res.get('error')}")

                    self.current_agent = None
                    # Short pause after completing task
                    await asyncio.sleep(1.0)
                else:
                    # Low-power idle sleep when no tasks are queued (5s)
                    await asyncio.sleep(5.0)

            except Exception as e:
                logger.error(f"Error in daemon supervisor cycle: {e}", exc_info=True)
                await asyncio.sleep(5.0)

        self.remove_pid()
        logger.info("Daemon gracefully terminated.")

    def _seed_default_schedules(self) -> None:
        """Seed useful recurring maintenance routines if table is empty."""
        existing = self.storage.list_schedules()
        if not existing:
            # 1. Daily system cleanup & status report
            self.storage.add_schedule(
                name="Daily System Health & Cache Audit",
                goal="Read system clipboard, inspect disk space with df -h, and log report",
                cron_expr="0 0 * * *",
                priority=TaskPriority.LOW
            )
            # 2. Hourly desktop organization
            self.storage.add_schedule(
                name="Hourly Telemetry Check",
                goal="Check running applications and system status",
                interval_seconds=3600,
                priority=TaskPriority.LOW
            )

