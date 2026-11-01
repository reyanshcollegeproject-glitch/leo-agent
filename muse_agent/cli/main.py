from __future__ import annotations

import asyncio
import os
import sys
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
    mock: bool = typer.Option(True, "--mock/--live", help="Use deterministic mock model or live Meta Muse API"),
    output: str = typer.Option("benchmark_results", "--output", "-o", help="Output directory for reports")
):
    """
    Run the standardized benchmark suite to evaluate OS control and set new agent benchmarks.
    """
    runner = BenchmarkRunner(use_mock=mock, output_dir=settings.benchmark_dir)
    asyncio.run(runner.run_suite())


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


if __name__ == "__main__":
    main()
