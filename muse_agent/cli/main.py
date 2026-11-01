from __future__ import annotations

import asyncio
import os
import signal
import sys
from pathlib import Path
from typing import Optional
import typer
import uvicorn
from rich.console import Console
from rich.panel import Panel
from rich.table import Table

from muse_agent.benchmark.runner import BenchmarkRunner
from muse_agent.config import settings
from muse_agent.core.agent import MuseAgent
from muse_agent.models.meta_muse import MetaMuseModel
from muse_agent.models.mock_model import MockMuseModel
from muse_agent.perception.grounding import VisualGrounder
from muse_agent.perception.screen import ScreenPerceiver

app = typer.Typer(
    name="muse-agent",
    help="Meta Muse Production Autonomous Laptop Control Agent CLI",
    add_completion=False
)
console = Console()


@app.command()
def run(
    goal: str = typer.Argument(..., help="The autonomous objective for the agent to execute on your laptop"),
    max_steps: int = typer.Option(25, "--max-steps", "-s", help="Maximum steps allowed"),
    mock: bool = typer.Option(False, "--mock", help="Run with deterministic offline mock model"),
    grid: bool = typer.Option(True, "--grid/--no-grid", help="Overlay coordinate grid on screen reasoning")
):
    """
    Execute an autonomous goal on your laptop using Meta AI Muse.
    """
    console.print(Panel(
        f"[bold cyan]Meta Muse Autonomous Laptop Agent[/bold cyan]\n"
        f"Goal: [bold yellow]\"{goal}\"[/bold yellow]\n"
        f"Model: [magenta]{'Offline Mock' if mock else settings.muse_model_name}[/magenta] | Max Steps: [green]{max_steps}[/green]",
        title="🤖 Agent Session Initialized",
        border_style="cyan"
    ))

    # Initialize model
    if mock:
        model = MockMuseModel()
    else:
        if not settings.muse_api_key:
            console.print(
                "[bold red]Warning:[/bold red] MUSE_API_KEY is not set in environment or .env!\n"
                "Tip: You can run with [bold cyan]--mock[/bold cyan] for offline dry runs or set your key in .env\n"
            )
            if not typer.confirm("Continue with Mock Simulation?"):
                sys.exit(1)
            model = MockMuseModel()
        else:
            model = MetaMuseModel(
                api_key=settings.muse_api_key,
                model_name=settings.muse_model_name,
                endpoint=settings.muse_endpoint
            )

    settings.use_grid_overlay = grid
    agent = MuseAgent(model=model)

    # Attach live CLI telemetry logger
    def cli_telemetry_listener(payload):
        step = payload["step"]
        action = payload["action"]
        res = payload["result"]
        diff = payload.get("visual_diff")

        act_type = action.get("action", "")
        thought = action.get("thought", "")

        console.print(f"[bold cyan]Step #{step}[/bold cyan] ➔ [bold magenta]{act_type.upper()}[/bold magenta]")
        if thought:
            console.print(f"  [dim]Thought:[/dim] {thought}")
        console.print(f"  [green]Result:[/green] {res.get('message')}")
        if diff and diff.get("has_changed"):
            console.print(f"  [blue]Visual Verification:[/blue] {diff.get('description')}")
        console.print("")

    agent.add_step_callback(cli_telemetry_listener)

    # Execute
    res = asyncio.run(agent.run(goal=goal, max_steps=max_steps))

    if res.get("success"):
        console.print(Panel(
            f"[bold green]Task Succeeded![/bold green]\n"
            f"Steps: {res.get('total_steps')} | Elapsed: {res.get('elapsed_seconds')}s\n"
            f"Trajectory saved to: [cyan]{res.get('trajectory_file')}[/cyan]",
            border_style="green"
        ))
    else:
        console.print(Panel(
            f"[bold red]Task Did Not Complete Successfully[/bold red]\n"
            f"State: {res.get('state')} | Error: {res.get('error', 'Limit exceeded')}",
            border_style="red"
        ))


@app.command()
def bench(
    live: bool = typer.Option(True, "--live/--mock", help="Use live Meta Muse API or offline simulation"),
    output: str = typer.Option("benchmark_results", "--output", "-o", help="Output directory for reports")
):
    """
    Run the standardized benchmark suite to evaluate OS control and set new agent benchmarks.
    """
    use_mock = not live
    if live and not settings.muse_api_key:
        console.print("[bold yellow]No MUSE_API_KEY detected in .env. Falling back to offline simulation.[/bold yellow]")
        use_mock = True

    runner = BenchmarkRunner(use_mock=use_mock, output_dir=settings.benchmark_dir)
    asyncio.run(runner.run_suite())


@app.command("desktop")
def desktop_command(
    mock: bool = typer.Option(False, "--mock", help="Run with offline simulation model")
):
    """
    Launch native macOS Desktop Control Center & Floating HUD (No browser/web server).
    """
    from muse_agent.desktop.hud import launch_desktop_hud
    console.print(Panel(
        f"[bold cyan]Launching Native macOS Desktop Control HUD[/bold cyan]\n"
        f"Mode: [green]Pure Desktop Native Window (No Browser)[/green]\n"
        f"Model: [magenta]{'Offline Mock' if mock else settings.muse_model_name}[/magenta]",
        border_style="cyan"
    ))
    launch_desktop_hud(mock=mock)


@app.command("gui")
def gui_alias(
    mock: bool = typer.Option(False, "--mock", help="Run with offline simulation model")
):
    """Alias for 'desktop': Launch native macOS Desktop Control Center."""
    desktop_command(mock=mock)


@app.command()
def serve(
    host: str = typer.Option("127.0.0.1", "--host", "-h", help="Host address to bind"),
    port: int = typer.Option(8765, "--port", "-p", help="Port to bind server")
):
    """
    Launch the real-time Web Control Center and Telemetry Dashboard.
    """
    console.print(Panel(
        f"[bold cyan]Starting Meta Muse Real-Time Web Observer[/bold cyan]\n"
        f"Dashboard URL: [bold green]http://{host}:{port}[/bold green]\n"
        f"WebSocket Endpoint: [dim]ws://{host}:{port}/ws[/dim]",
        border_style="cyan"
    ))
    uvicorn.run("muse_agent.server.app:app", host=host, port=port, reload=False)


@app.command()
def inspect():
    """
    Inspect laptop screen resolution, Retina factor, and detect UI candidate bounding boxes.
    """
    perceiver = ScreenPerceiver()
    info = perceiver.info
    table = Table(title="🖥️ Laptop Display & Screen Geometry")
    table.add_column("Property", style="cyan")
    table.add_column("Value", style="green")

    table.add_row("Logical Resolution", f"{info.logical_width} x {info.logical_height}")
    table.add_row("Physical Pixel Resolution", f"{info.physical_width} x {info.physical_height}")
    table.add_row("Retina Scale Factor", f"{info.scale_factor}x")

    console.print(table)

    # Capture test screenshot
    img = perceiver.capture()
    grounder = VisualGrounder()
    candidates = grounder.detect_ui_candidates(img)
    console.print(f"\n[bold green]Detected {len(candidates)} active interactive UI regions via Set-of-Marks detector.[/bold green]")
    for c in candidates[:5]:
        console.print(f"  - Element [bold cyan]#{c['id']}[/bold cyan] at norm coords ({c['cx']}, {c['cy']})")


def main():
    app()


# --- 24/7 AUTONOMOUS DAEMON COMMANDS ---
daemon_app = typer.Typer(help="Manage 24/7 background autonomous laptop daemon")
queue_app = typer.Typer(help="Manage persistent autonomous task queue")
schedule_app = typer.Typer(help="Manage recurring cron & interval schedules")
app.add_typer(daemon_app, name="daemon")
app.add_typer(queue_app, name="queue")
app.add_typer(schedule_app, name="schedule")


@daemon_app.command("start")
def daemon_start(
    background: bool = typer.Option(False, "--bg", "-b", help="Fork into detached background process")
):
    """
    Start the 24/7 autonomous daemon on your laptop.
    """
    from muse_agent.daemon.service import AutonomousDaemon

    if background:
        console.print("[bold cyan]Forking Meta Muse Daemon to background...[/bold cyan]")
        import subprocess
        log_out = open("logs/daemon.log", "a")
        proc = subprocess.Popen(
            [sys.executable, "-m", "muse_agent.cli.main", "daemon", "start"],
            stdout=log_out,
            stderr=log_out,
            start_new_session=True
        )
        console.print(f"[bold green]Daemon launched in background with PID {proc.pid}[/bold green]")
        console.print("View live logs via: [cyan]tail -f logs/daemon.log[/cyan]")
        return

    console.print(Panel(
        f"[bold cyan]Starting Meta Muse 24/7 Autonomous Laptop Daemon[/bold cyan]\n"
        f"Model: [magenta]{settings.muse_model_name}[/magenta] | Uptime: Continuous\n"
        f"Safety: Sentinel Active | Power Governor: Enabled\n"
        f"[dim]Press Ctrl+C to halt daemon[/dim]",
        border_style="cyan"
    ))
    daemon = AutonomousDaemon()
    asyncio.run(daemon.start())


@daemon_app.command("stop")
def daemon_stop():
    """
    Stop the running 24/7 autonomous daemon.
    """
    pid_file = Path("data/daemon.pid")
    if not pid_file.exists():
        console.print("[bold yellow]No running daemon PID file found.[/bold yellow]")
        return

    pid = int(pid_file.read_text().strip())
    try:
        os.kill(pid, signal.SIGTERM)
        console.print(f"[bold green]Sent stop signal to daemon PID {pid}.[/bold green]")
        pid_file.unlink()
    except ProcessLookupError:
        console.print(f"[bold yellow]Process {pid} was not running. Cleared stale PID file.[/bold yellow]")
        pid_file.unlink()
    except Exception as e:
        console.print(f"[bold red]Failed to stop daemon: {e}[/bold red]")


@daemon_app.command("status")
def daemon_status():
    """
    View 24/7 daemon health, system load, uptime, and queue state.
    """
    from muse_agent.daemon.storage import DaemonStorage
    from muse_agent.daemon.watchers import SystemWatcher
    from muse_agent.daemon.launchd import LaunchdManager

    storage = DaemonStorage()
    watcher = SystemWatcher()
    snap = watcher.capture_snapshot()
    hb = storage.get_latest_heartbeat()
    launchd = LaunchdManager().get_status()

    pid_file = Path("data/daemon.pid")
    is_running = pid_file.exists()
    pid = pid_file.read_text().strip() if is_running else "None"

    table = Table(title="🤖 Meta Muse 24/7 Autonomous Daemon Status")
    table.add_column("Property", style="cyan")
    table.add_column("Value", style="green")

    table.add_row("Daemon Running", "🟢 ACTIVE" if is_running else "⚪ STOPPED")
    table.add_row("Process PID", pid)
    table.add_row("macOS launchd Installed", "✅ YES" if launchd["installed"] else "❌ NO")
    table.add_row("Model", settings.muse_model_name)
    table.add_row("CPU Load", f"{snap.cpu_percent}%")
    table.add_row("Memory Used", f"{snap.memory_percent}%")
    table.add_row("Free Disk Space", f"{snap.disk_free_gb} GB")
    table.add_row("Battery", f"{snap.battery_percent}% ({'Charging ⚡' if snap.is_charging else 'On Battery 🔋'})")
    table.add_row("User Idle Time", f"{snap.user_idle_seconds}s ({'User Active' if snap.is_user_active else 'User Away'})")

    if hb:
        table.add_row("Uptime", f"{int(hb['uptime_seconds'] // 3600)}h {int((hb['uptime_seconds'] % 3600) // 60)}m")

    console.print(table)

    # Queue summary
    tasks = storage.list_tasks(limit=5)
    pending = sum(1 for t in tasks if t["status"] == "pending")
    completed = sum(1 for t in tasks if t["status"] == "completed")
    console.print(f"\n[bold]Queue Summary:[/bold] {pending} pending | {completed} completed")


@daemon_app.command("install")
def daemon_install():
    """
    Register daemon as native macOS launchd service to run 24/7 on system startup.
    """
    from muse_agent.daemon.launchd import LaunchdManager
    mgr = LaunchdManager()
    ok = mgr.install()
    if ok:
        console.print(Panel(
            "[bold green]Successfully installed 24/7 macOS LaunchAgent![/bold green]\n"
            f"Service: [cyan]{mgr.SERVICE_LABEL}[/cyan]\n"
            f"Config: [dim]{mgr.plist_path}[/dim]\n"
            "The autonomous agent will now automatically run 24/7 and auto-restart on boot.",
            border_style="green"
        ))
    else:
        console.print("[bold red]Failed to install launchd service. Check permissions.[/bold red]")


@daemon_app.command("uninstall")
def daemon_uninstall():
    """
    Remove daemon from macOS launchd startup services.
    """
    from muse_agent.daemon.launchd import LaunchdManager
    mgr = LaunchdManager()
    mgr.uninstall()
    console.print("[bold green]Successfully removed 24/7 LaunchAgent service.[/bold green]")


# --- TASK QUEUE COMMANDS ---
@queue_app.command("add")
def queue_add(
    goal: str = typer.Argument(..., help="The autonomous objective to enqueue"),
    priority: str = typer.Option("normal", "--priority", "-p", help="Task priority: critical, high, normal, low")
):
    """
    Add a task to the 24/7 autonomous daemon queue.
    """
    from muse_agent.daemon.storage import DaemonStorage, TaskPriority
    storage = DaemonStorage()
    p = TaskPriority(priority.lower())
    task_id = storage.add_task(goal=goal, priority=p)
    console.print(f"[bold green]Enqueued Task #{task_id}[/bold green] [Priority: [yellow]{p.value}[/yellow]]: \"{goal}\"")


@queue_app.command("list")
def queue_list(limit: int = typer.Option(20, "--limit", "-n")):
    """
    List tasks in the persistent queue.
    """
    from muse_agent.daemon.storage import DaemonStorage
    storage = DaemonStorage()
    tasks = storage.list_tasks(limit=limit)

    table = Table(title="📋 Persistent Autonomous Task Queue")
    table.add_column("ID", style="cyan")
    table.add_column("Priority", style="yellow")
    table.add_column("Status", style="bold")
    table.add_column("Goal", style="white")
    table.add_column("Result / Error", style="dim")

    for t in tasks:
        status_color = "green" if t["status"] == "completed" else ("yellow" if t["status"] == "pending" else "red")
        table.add_row(
            str(t["id"]),
            t["priority"],
            f"[{status_color}]{t['status'].upper()}[/{status_color}]",
            t["goal"],
            t.get("result_summary") or t.get("error") or ""
        )

    console.print(table)


# --- RECURRING SCHEDULE COMMANDS ---
@schedule_app.command("add")
def schedule_add(
    name: str = typer.Argument(..., help="Descriptive name for the recurring job"),
    goal: str = typer.Argument(..., help="Objective to execute on schedule"),
    cron: Optional[str] = typer.Option(None, "--cron", "-c", help="Cron expression e.g. '0 0 * * *' (midnight)"),
    interval: Optional[int] = typer.Option(None, "--interval", "-i", help="Interval in seconds (e.g. 3600 for hourly)")
):
    """
    Add a recurring autonomous schedule (cron or interval).
    """
    from muse_agent.daemon.storage import DaemonStorage
    storage = DaemonStorage()
    sched_id = storage.add_schedule(name=name, goal=goal, cron_expr=cron, interval_seconds=interval)
    console.print(f"[bold green]Created Schedule #{sched_id}[/bold green] '{name}': \"{goal}\"")


@schedule_app.command("list")
def schedule_list():
    """
    List all configured recurring autonomous routines.
    """
    from muse_agent.daemon.storage import DaemonStorage
    from datetime import datetime
    storage = DaemonStorage()
    scheds = storage.list_schedules()

    table = Table(title="⏰ 24/7 Recurring Autonomous Routines")
    table.add_column("ID", style="cyan")
    table.add_column("Name", style="magenta")
    table.add_column("Goal", style="white")
    table.add_column("Trigger", style="yellow")
    table.add_column("Next Execution", style="green")

    for s in scheds:
        trigger = s.get("cron_expr") or f"Every {s.get('interval_seconds')}s"
        next_dt = datetime.fromtimestamp(s["next_run"]).strftime("%Y-%m-%d %H:%M:%S")
        table.add_row(str(s["id"]), s["name"], s["goal"], trigger, next_dt)

    console.print(table)


if __name__ == "__main__":
    main()

