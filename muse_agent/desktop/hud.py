from __future__ import annotations

import asyncio
import queue
import threading
import time
import tkinter as tk
from tkinter import ttk, messagebox
from typing import Optional

from muse_agent.config import settings
from muse_agent.core.agent import AgentState, MuseAgent
from muse_agent.daemon.storage import DaemonStorage, TaskPriority
from muse_agent.models.meta_muse import MetaMuseModel
from muse_agent.models.mock_model import MockMuseModel


class DesktopHUD:
    """
    Native macOS Desktop Control Center & Floating HUD.
    Provides direct, real-time desktop control without needing any browser or web server.
    """

    def __init__(self, use_mock: bool = False):
        self.use_mock = use_mock
        self.root = tk.Tk()
        self.root.title("⚡ Meta Muse Desktop Control")
        self.root.geometry("560x480")
        self.root.minsize(480, 400)

        # Dark theme styling
        self.bg_color = "#0e1117"
        self.card_color = "#161b26"
        self.accent_cyan = "#00e5ff"
        self.accent_green = "#00e676"
        self.accent_red = "#ff3366"
        self.accent_orange = "#ff9100"
        self.text_color = "#f0f4fc"
        self.text_muted = "#8b9bb4"

        self.root.configure(bg=self.bg_color)

        # Thread-safe event queue for agent callbacks
        self.event_queue: queue.Queue = queue.Queue()
        self.agent: Optional[MuseAgent] = None
        self.agent_thread: Optional[threading.Thread] = None
        self.step_count = 0

        self._build_ui()
        self._poll_event_queue()

    def _build_ui(self) -> None:
        # 1. Top Header Bar
        header = tk.Frame(self.root, bg=self.card_color, height=44, padx=14, pady=8)
        header.pack(fill=tk.X, side=tk.TOP)

        title_lbl = tk.Label(
            header,
            text="⚡ META MUSE DESKTOP",
            font=("Helvetica", 13, "bold"),
            fg=self.accent_cyan,
            bg=self.card_color
        )
        title_lbl.pack(side=tk.LEFT)

        self.topmost_var = tk.BooleanVar(value=True)
        self.root.attributes("-topmost", True)

        chk_top = tk.Checkbutton(
            header,
            text="Pin on Top",
            variable=self.topmost_var,
            command=self._toggle_topmost,
            bg=self.card_color,
            fg=self.text_muted,
            selectcolor=self.bg_color,
            activebackground=self.card_color,
            font=("Helvetica", 10)
        )
        chk_top.pack(side=tk.RIGHT, padx=6)

        self.state_badge = tk.Label(
            header,
            text="● IDLE",
            font=("Helvetica", 10, "bold"),
            fg=self.accent_green,
            bg=self.card_color
        )
        self.state_badge.pack(side=tk.RIGHT, padx=10)

        # 2. Goal Input Section
        input_frame = tk.Frame(self.root, bg=self.bg_color, padx=14, pady=10)
        input_frame.pack(fill=tk.X)

        input_lbl = tk.Label(
            input_frame,
            text="AUTONOMOUS DESKTOP GOAL:",
            font=("Helvetica", 10, "bold"),
            fg=self.text_muted,
            bg=self.bg_color
        )
        input_lbl.pack(anchor=tk.W, pady=(0, 4))

        entry_row = tk.Frame(input_frame, bg=self.bg_color)
        entry_row.pack(fill=tk.X)

        self.goal_entry = tk.Entry(
            entry_row,
            font=("Helvetica", 12),
            bg=self.card_color,
            fg=self.text_color,
            insertbackground=self.accent_cyan,
            relief=tk.FLAT,
            bd=6
        )
        self.goal_entry.pack(side=tk.LEFT, fill=tk.X, expand=True, padx=(0, 8))
        self.goal_entry.insert(0, "Open Calculator and calculate 128 * 64")
        self.goal_entry.bind("<Return>", lambda e: self.start_task())

        self.btn_run = tk.Button(
            entry_row,
            text="⚡ Run",
            font=("Helvetica", 11, "bold"),
            bg=self.accent_cyan,
            fg="#000",
            relief=tk.FLAT,
            padx=16,
            pady=4,
            command=self.start_task
        )
        self.btn_run.pack(side=tk.RIGHT)

        # 3. Control Buttons Row
        btn_row = tk.Frame(self.root, bg=self.bg_color, padx=14, pady=4)
        btn_row.pack(fill=tk.X)

        self.btn_pause = tk.Button(
            btn_row,
            text="⏸️ Pause",
            font=("Helvetica", 10),
            bg=self.card_color,
            fg=self.text_color,
            relief=tk.FLAT,
            padx=10,
            command=self.pause_task,
            state=tk.DISABLED
        )
        self.btn_pause.pack(side=tk.LEFT, padx=(0, 6))

        self.btn_resume = tk.Button(
            btn_row,
            text="▶️ Resume",
            font=("Helvetica", 10),
            bg=self.card_color,
            fg=self.text_color,
            relief=tk.FLAT,
            padx=10,
            command=self.resume_task,
            state=tk.DISABLED
        )
        self.btn_resume.pack(side=tk.LEFT, padx=(0, 6))

        self.btn_queue = tk.Button(
            btn_row,
            text="📋 Enqueue",
            font=("Helvetica", 10),
            bg=self.card_color,
            fg=self.text_color,
            relief=tk.FLAT,
            padx=10,
            command=self.enqueue_task
        )
        self.btn_queue.pack(side=tk.LEFT, padx=(0, 6))

        self.btn_stop = tk.Button(
            btn_row,
            text="🛑 EMERGENCY STOP",
            font=("Helvetica", 10, "bold"),
            bg=self.accent_red,
            fg="#fff",
            relief=tk.FLAT,
            padx=12,
            command=self.stop_task,
            state=tk.DISABLED
        )
        self.btn_stop.pack(side=tk.RIGHT)

        # 4. Thought Card
        thought_frame = tk.Frame(self.root, bg=self.card_color, padx=12, pady=10, bd=1, relief=tk.SOLID)
        thought_frame.pack(fill=tk.X, padx=14, pady=8)

        thought_tag = tk.Label(
            thought_frame,
            text="CURRENT REASONING & THOUGHT:",
            font=("Helvetica", 9, "bold"),
            fg=self.accent_cyan,
            bg=self.card_color
        )
        thought_tag.pack(anchor=tk.W)

        self.thought_lbl = tk.Label(
            thought_frame,
            text="Ready. Enter a desktop goal above and click 'Run'.",
            font=("Helvetica", 11),
            fg=self.text_color,
            bg=self.card_color,
            wraplength=520,
            justify=tk.LEFT
        )
        self.thought_lbl.pack(anchor=tk.W, pady=(4, 0))

        # 5. Action History Log
        history_frame = tk.Frame(self.root, bg=self.bg_color, padx=14, pady=4)
        history_frame.pack(fill=tk.BOTH, expand=True)

        history_tag = tk.Label(
            history_frame,
            text="ACTION EXECUTION FEED:",
            font=("Helvetica", 9, "bold"),
            fg=self.text_muted,
            bg=self.bg_color
        )
        history_tag.pack(anchor=tk.W, pady=(0, 4))

        self.log_text = tk.Text(
            history_frame,
            bg=self.card_color,
            fg=self.text_color,
            font=("Courier", 10),
            bd=0,
            padx=8,
            pady=8,
            wrap=tk.WORD
        )
        self.log_text.pack(fill=tk.BOTH, expand=True)

        # 6. Bottom Status Ribbon
        status_bar = tk.Frame(self.root, bg=self.card_color, height=24, padx=14, pady=4)
        status_bar.pack(fill=tk.X, side=tk.BOTTOM)

        self.model_lbl = tk.Label(
            status_bar,
            text=f"Model: {settings.muse_model_name}",
            font=("Helvetica", 9),
            fg=self.text_muted,
            bg=self.card_color
        )
        self.model_lbl.pack(side=tk.LEFT)

        self.step_lbl = tk.Label(
            status_bar,
            text="Steps: 0",
            font=("Helvetica", 9),
            fg=self.accent_cyan,
            bg=self.card_color
        )
        self.step_lbl.pack(side=tk.RIGHT)

    def _toggle_topmost(self) -> None:
        self.root.attributes("-topmost", self.topmost_var.get())

    def _poll_event_queue(self) -> None:
        """Check for events sent from the agent worker thread."""
        while not self.event_queue.empty():
            evt = self.event_queue.get()
            t = evt.get("type")
            if t == "step":
                self._handle_step_event(evt["data"])
            elif t == "finished":
                self._handle_finished_event(evt["data"])
            elif t == "state":
                self._update_state_ui(evt["state"])

        self.root.after(80, self._poll_event_queue)

    def _update_state_ui(self, state: str) -> None:
        s = state.upper()
        if s == "RUNNING":
            self.state_badge.config(text=f"● {s}", fg=self.accent_cyan)
            self.btn_run.config(state=tk.DISABLED)
            self.btn_pause.config(state=tk.NORMAL)
            self.btn_resume.config(state=tk.DISABLED)
            self.btn_stop.config(state=tk.NORMAL)
        elif s == "PAUSED":
            self.state_badge.config(text=f"● {s}", fg=self.accent_orange)
            self.btn_pause.config(state=tk.DISABLED)
            self.btn_resume.config(state=tk.NORMAL)
        elif s == "COMPLETED":
            self.state_badge.config(text=f"● {s}", fg=self.accent_green)
            self.btn_run.config(state=tk.NORMAL)
            self.btn_pause.config(state=tk.DISABLED)
            self.btn_resume.config(state=tk.DISABLED)
            self.btn_stop.config(state=tk.DISABLED)
        else:
            self.state_badge.config(text=f"● {s}", fg=self.accent_red if s in ("STOPPED", "FAILED") else self.text_muted)
            self.btn_run.config(state=tk.NORMAL)
            self.btn_pause.config(state=tk.DISABLED)
            self.btn_resume.config(state=tk.DISABLED)
            self.btn_stop.config(state=tk.DISABLED)

    def _handle_step_event(self, data: dict) -> None:
        self.step_count = data.get("step", 0)
        self.step_lbl.config(text=f"Steps: {self.step_count}")

        action = data.get("action", {})
        result = data.get("result", {})
        thought = action.get("thought", "")

        if thought:
            self.thought_lbl.config(text=thought)

        act_type = action.get("action", "").upper()
        res_msg = result.get("message", "Executed")

        log_line = f"[Step #{self.step_count}] {act_type} ➔ {res_msg}\n"
        self.log_text.insert(tk.END, log_line)
        self.log_text.see(tk.END)

    def _handle_finished_event(self, data: dict) -> None:
        self._update_state_ui("COMPLETED" if data.get("success") else "FAILED")
        summary = f"\n=== Task {'Succeeded 🎉' if data.get('success') else 'Terminated ⚠️'} ({data.get('elapsed_seconds')}s, {data.get('total_steps')} steps) ===\n"
        self.log_text.insert(tk.END, summary)
        self.log_text.see(tk.END)
        self.thought_lbl.config(text=f"Task completed in {data.get('elapsed_seconds')}s across {data.get('total_steps')} steps.")

    def start_task(self) -> None:
        goal = self.goal_entry.get().strip()
        if not goal:
            messagebox.showwarning("Empty Goal", "Please enter a desktop goal.")
            return

        self.log_text.delete("1.0", tk.END)
        self.step_count = 0
        self.step_lbl.config(text="Steps: 0")
        self._update_state_ui("RUNNING")
        self.thought_lbl.config(text="Initializing OODA cognitive loop and analyzing screen...")

        def run_worker():
            if self.use_mock:
                model = MockMuseModel()
            else:
                model = MetaMuseModel(
                    api_key=settings.muse_api_key,
                    model_name=settings.muse_model_name,
                    endpoint=settings.muse_endpoint
                )

            self.agent = MuseAgent(model=model, run_id=f"desktop_{int(time.time())}")

            def on_step(payload):
                self.event_queue.put({"type": "step", "data": payload})

            self.agent.add_step_callback(on_step)

            res = asyncio.run(self.agent.run(goal=goal, max_steps=25))
            self.event_queue.put({"type": "finished", "data": res})

        self.agent_thread = threading.Thread(target=run_worker, daemon=True)
        self.agent_thread.start()

    def pause_task(self) -> None:
        if self.agent:
            self.agent.pause()
            self._update_state_ui("PAUSED")

    def resume_task(self) -> None:
        if self.agent:
            self.agent.resume()
            self._update_state_ui("RUNNING")

    def stop_task(self) -> None:
        if self.agent:
            self.agent.stop()
            self._update_state_ui("STOPPED")
            self.thought_lbl.config(text="Emergency Kill Switch Triggered. Agent Halted.")

    def enqueue_task(self) -> None:
        goal = self.goal_entry.get().strip()
        if not goal:
            return
        storage = DaemonStorage()
        task_id = storage.add_task(goal=goal, priority=TaskPriority.HIGH)
        messagebox.showinfo("Enqueued", f"Task #{task_id} enqueued into 24/7 background queue.")

    def run(self) -> None:
        self.root.mainloop()


def launch_desktop_hud(mock: bool = False) -> None:
    hud = DesktopHUD(use_mock=mock)
    hud.run()


if __name__ == "__main__":
    launch_desktop_hud()
