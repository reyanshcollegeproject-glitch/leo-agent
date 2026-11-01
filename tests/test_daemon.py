import time
from pathlib import Path
from muse_agent.daemon.storage import DaemonStorage, TaskPriority, TaskStatus
from muse_agent.daemon.watchers import SystemWatcher
from muse_agent.daemon.scheduler import AutonomousScheduler
from muse_agent.daemon.launchd import LaunchdManager


def test_daemon_storage_task_queue(tmp_path: Path):
    db_file = tmp_path / "test_agent.db"
    storage = DaemonStorage(db_path=db_file)

    # Add low and high priority tasks
    t1 = storage.add_task("Low priority task", priority=TaskPriority.LOW)
    t2 = storage.add_task("High priority task", priority=TaskPriority.HIGH)

    # Highest priority should be retrieved first
    next_task = storage.fetch_next_pending_task()
    assert next_task is not None
    assert next_task.id == t2
    assert next_task.priority == TaskPriority.HIGH

    # Update status to completed
    storage.update_task_status(t2, TaskStatus.COMPLETED, result_summary="Done")
    next_task2 = storage.fetch_next_pending_task()
    assert next_task2.id == t1


def test_daemon_storage_heartbeat(tmp_path: Path):
    db_file = tmp_path / "test_agent.db"
    storage = DaemonStorage(db_path=db_file)

    storage.record_heartbeat(
        uptime_seconds=1200.0,
        cpu_percent=12.5,
        memory_percent=60.0,
        battery_percent=85.0,
        is_charging=True,
        user_idle_seconds=300.0,
        daemon_status="running"
    )

    hb = storage.get_latest_heartbeat()
    assert hb is not None
    assert hb["uptime_seconds"] == 1200.0
    assert hb["battery_percent"] == 85.0


def test_system_watcher():
    watcher = SystemWatcher(idle_threshold_seconds=100.0)
    snap = watcher.capture_snapshot()
    assert 0 <= snap.cpu_percent <= 100
    assert 0 <= snap.memory_percent <= 100
    assert snap.disk_free_gb > 0
    assert 0 <= snap.battery_percent <= 100


def test_autonomous_scheduler(tmp_path: Path):
    db_file = tmp_path / "test_agent.db"
    storage = DaemonStorage(db_path=db_file)
    scheduler = AutonomousScheduler(storage)

    # Add an interval schedule that is already due
    now = time.time()
    s_id = storage.add_schedule(
        name="Test Interval",
        goal="Run interval maintenance",
        interval_seconds=10
    )
    # Manually set next_run in the past
    with storage._get_conn() as conn:
        conn.execute("UPDATE schedules SET next_run = ? WHERE id = ?", (now - 10, s_id))

    enqueued = scheduler.poll_and_enqueue_due_jobs()
    assert len(enqueued) == 1
    task = storage.fetch_next_pending_task()
    assert task is not None
    assert "Run interval maintenance" in task.goal


def test_launchd_plist_generation():
    mgr = LaunchdManager()
    plist_dict = mgr.generate_plist_dict()
    assert plist_dict["Label"] == LaunchdManager.SERVICE_LABEL
    assert plist_dict["RunAtLoad"] is True
    assert "KeepAlive" in plist_dict

