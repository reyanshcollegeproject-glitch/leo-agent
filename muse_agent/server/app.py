from __future__ import annotations

import asyncio
import io
import json
import logging
from pathlib import Path
from typing import Any, Dict, List, Optional
from fastapi import FastAPI, WebSocket, WebSocketDisconnect, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import FileResponse, HTMLResponse
from fastapi.staticfiles import StaticFiles
from pydantic import BaseModel

from muse_agent.config import settings
from muse_agent.core.agent import AgentState, MuseAgent
from muse_agent.models.meta_muse import MetaMuseModel
from muse_agent.models.mock_model import MockMuseModel

logger = logging.getLogger("muse_agent.server")

app = FastAPI(
    title="Meta Muse Autonomous Laptop Agent Server",
    description="Real-time control center, telemetry streaming, and REST API for autonomous OS control.",
    version="1.0.0"
)

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Global Agent Reference & Connection Manager
current_agent: Optional[MuseAgent] = None
active_task: Optional[asyncio.Task] = None


class ConnectionManager:
    def __init__(self):
        self.active_connections: List[WebSocket] = []

    async def connect(self, websocket: WebSocket):
        await websocket.accept()
        self.active_connections.append(websocket)

    def disconnect(self, websocket: WebSocket):
        if websocket in self.active_connections:
            self.active_connections.remove(websocket)

    async def broadcast(self, message: Dict[str, Any]):
        dead = []
        for conn in self.active_connections:
            try:
                await conn.send_json(message)
            except Exception:
                dead.append(conn)
        for d in dead:
            if d in self.active_connections:
                self.active_connections.remove(d)


manager = ConnectionManager()


class RunTaskRequest(BaseModel):
    goal: str
    max_steps: Optional[int] = 25
    use_mock: Optional[bool] = False


@app.get("/api/status")
async def get_status():
    global current_agent
    if current_agent:
        return {
            "state": current_agent.state.value,
            "current_goal": current_agent.current_goal,
            "steps_count": len(current_agent.memory.steps),
            "run_id": current_agent.memory.run_id
        }
    return {"state": AgentState.IDLE.value, "current_goal": None, "steps_count": 0}


@app.post("/api/run")
async def run_task(req: RunTaskRequest):
    global current_agent, active_task

    if current_agent and current_agent.state == AgentState.RUNNING:
        raise HTTPException(status_code=400, detail="An agent task is already running!")

    if req.use_mock:
        model = MockMuseModel()
    else:
        model = MetaMuseModel(
            api_key=settings.muse_api_key,
            model_name=settings.muse_model_name,
            endpoint=settings.muse_endpoint
        )

    current_agent = MuseAgent(model=model)

    # Attach live websocket telemetry callback
    def on_step(payload: Dict[str, Any]):
        asyncio.create_task(manager.broadcast({"type": "step_telemetry", "data": payload}))

    current_agent.add_step_callback(on_step)

    async def runner():
        res = await current_agent.run(goal=req.goal, max_steps=req.max_steps)
        await manager.broadcast({"type": "task_finished", "data": res})

    active_task = asyncio.create_task(runner())
    return {"status": "started", "goal": req.goal, "run_id": current_agent.memory.run_id}


@app.post("/api/pause")
async def pause_task():
    global current_agent
    if current_agent:
        current_agent.pause()
        await manager.broadcast({"type": "state_change", "state": current_agent.state.value})
        return {"status": "paused"}
    return {"status": "no_active_agent"}


@app.post("/api/resume")
async def resume_task():
    global current_agent
    if current_agent:
        current_agent.resume()
        await manager.broadcast({"type": "state_change", "state": current_agent.state.value})
        return {"status": "resumed"}
    return {"status": "no_active_agent"}


@app.post("/api/stop")
async def stop_task():
    global current_agent, active_task
    if current_agent:
        current_agent.stop()
        if active_task and not active_task.done():
            active_task.cancel()
        await manager.broadcast({"type": "state_change", "state": AgentState.STOPPED.value})
        return {"status": "stopped"}
    return {"status": "no_active_agent"}


@app.get("/api/screenshot")
async def get_live_screenshot():
    """Captures and returns the current screen image."""
    global current_agent
    from fastapi.responses import Response
    from muse_agent.perception.screen import ScreenPerceiver

    perceiver = current_agent.perceiver if current_agent else ScreenPerceiver()
    b64, img = perceiver.capture_base64(max_width=1280, format="JPEG")
    buf = io.BytesIO()
    img.save(buf, format="JPEG", quality=80)
    return Response(content=buf.getvalue(), media_type="image/jpeg")


@app.websocket("/ws")
async def websocket_endpoint(websocket: WebSocket):
    await manager.connect(websocket)
    try:
        # Send initial status
        if current_agent:
            await websocket.send_json({
                "type": "initial_state",
                "state": current_agent.state.value,
                "goal": current_agent.current_goal,
                "steps": [s.action for s in current_agent.memory.steps]
            })
        while True:
            data = await websocket.receive_text()
            # Handle client commands if any
            cmd = json.loads(data)
            if cmd.get("action") == "ping":
                await websocket.send_json({"type": "pong"})
    except WebSocketDisconnect:
        manager.disconnect(websocket)


# Mount static directory
static_dir = Path(__file__).parent / "static"
if static_dir.exists():
    app.mount("/static", StaticFiles(directory=str(static_dir)), name="static")


@app.get("/")
async def get_index():
    index_file = static_dir / "index.html"
    if index_file.exists():
        return FileResponse(str(index_file))
    return HTMLResponse("<h1>Meta Muse Autonomous Agent API is Running</h1>")

