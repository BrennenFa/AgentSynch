#!/usr/bin/env python3
"""
AgentSynch TUI — live dashboard for tasks, agents, and reaper status.
Reads directly from the same SQLite database as the Go CLI.

Usage:
    python ui.py
"""

import sqlite3
import os
import time
from datetime import datetime, timezone

from rich.console import Console
from rich.table import Table
from rich.live import Live
from rich.panel import Panel
from rich.text import Text
from rich.columns import Columns
from rich import box

DB_PATH = os.path.expanduser("~/.agentsynch/tasks.db")
REFRESH_INTERVAL = 2       # seconds between UI refreshes
ZOMBIE_TIMEOUT   = 600     # 10 minutes — mirrors server.go zombieTimeout
REAP_INTERVAL    = 300     # 5 minutes  — mirrors server.go reapInterval

STATUS_COLORS = {
    "available":  "green",
    "claimed":    "yellow",
    "finished":   "blue",
    "validating": "cyan",
    "error":      "red",
    "blocked":    "bright_black",
    "archived":   "dim",
}


def rel_time(ts: str | None) -> str:
    if not ts:
        return "—"
    try:
        t = datetime.fromisoformat(ts.replace("Z", "+00:00"))
        ago = (datetime.now(timezone.utc) - t).total_seconds()
        if ago < 60:
            return f"{int(ago)}s ago"
        elif ago < 3600:
            return f"{int(ago // 60)}m ago"
        else:
            return f"{int(ago // 3600)}h ago"
    except Exception:
        return ts


def heartbeat_color(ts: str | None) -> str:
    """Returns a rich color based on how stale the heartbeat is."""
    if not ts:
        return "bright_black"
    try:
        t = datetime.fromisoformat(ts.replace("Z", "+00:00"))
        ago = (datetime.now(timezone.utc) - t).total_seconds()
        pct = ago / ZOMBIE_TIMEOUT
        if pct > 0.8:
            return "red"
        elif pct > 0.5:
            return "yellow"
        return "green"
    except Exception:
        return "white"


def agent_short(agent: str | None) -> str:
    if not agent:
        return "—"
    # strip "agent-" prefix to save space
    return agent.removeprefix("agent-")


def load_tasks(conn: sqlite3.Connection) -> list[dict]:
    conn.row_factory = sqlite3.Row
    cur = conn.execute("""
        SELECT id, title, status, claimed_by, claimed_at, created_at,
               finished_at, heartbeat_at, attempts
        FROM tasks
        WHERE status != 'archived'
        ORDER BY id
    """)
    return [dict(row) for row in cur.fetchall()]


def build_tasks_table(tasks: list[dict]) -> Table:
    table = Table(box=box.SIMPLE, show_header=True, header_style="bold")
    table.add_column("ID",        style="dim", width=4)
    table.add_column("Title",     width=28)
    table.add_column("Status",    width=12)
    table.add_column("Agent",     width=26)
    table.add_column("Created",   width=10)
    table.add_column("Claimed",   width=10)
    table.add_column("Heartbeat", width=12)
    table.add_column("Tries",     width=5)

    for t in tasks:
        status = t["status"]
        color  = STATUS_COLORS.get(status, "white")
        hb_color = heartbeat_color(t["heartbeat_at"])

        table.add_row(
            str(t["id"]),
            (t["title"] or "")[:26],
            Text(status, style=color),
            agent_short(t["claimed_by"])[:26],
            rel_time(t["created_at"]),
            rel_time(t["claimed_at"]),
            Text(rel_time(t["heartbeat_at"]), style=hb_color),
            str(t["attempts"]),
        )

    return table


def build_agents_panel(tasks: list[dict]) -> Panel:
    active = [t for t in tasks if t["status"] == "claimed" and t["claimed_by"]]
    if not active:
        content = Text("no active agents", style="bright_black")
    else:
        content = Text()
        for t in active:
            content.append(f"{t['claimed_by']}\n", style="cyan")
            content.append(f"  └─ task #{t['id']}: {t['title']}  (claimed {rel_time(t['claimed_at'])})\n")
    return Panel(content, title="[bold]Active Agents[/bold]", border_style="bright_black")


def build_reaper_panel(last_reap: float) -> Panel:
    now = time.time()
    since_reap = int(now - last_reap)
    next_reap  = max(0, REAP_INTERVAL - since_reap)

    content = Text()
    content.append(f"last reap:      {since_reap}s ago\n")
    content.append(f"next reap in:   {next_reap}s\n")
    content.append(f"reap interval:  {REAP_INTERVAL}s\n")
    content.append(f"zombie timeout: {ZOMBIE_TIMEOUT}s\n")
    return Panel(content, title="[bold]Reaper[/bold]", border_style="bright_black")


def build_layout(tasks: list[dict], last_reap: float) -> Table:
    # outer wrapper so rich Live can render a single renderable
    root = Table.grid(padding=(0, 0))
    root.add_row(f"[bold]AgentSynch Monitor[/bold]  [dim]{datetime.now().strftime('%Y-%m-%d %H:%M:%S')}[/dim]")
    root.add_row("")
    root.add_row("[bold]Tasks[/bold]")
    root.add_row(build_tasks_table(tasks))
    root.add_row(Columns([build_agents_panel(tasks), build_reaper_panel(last_reap)]))
    root.add_row(f"[dim]  refreshing every {REFRESH_INTERVAL}s — Ctrl+C to exit[/dim]")
    return root


def main():
    if not os.path.exists(DB_PATH):
        print(f"database not found at {DB_PATH}")
        print("run './agentsynch add' first to create it")
        return

    conn = sqlite3.connect(DB_PATH, check_same_thread=False)
    # enable WAL read so we don't block the Go writers
    conn.execute("PRAGMA journal_mode=WAL")

    last_reap = time.time()  # we don't track actual reap events, just approximate

    console = Console()
    try:
        with Live(console=console, refresh_per_second=1, screen=True) as live:
            while True:
                tasks = load_tasks(conn)
                live.update(build_layout(tasks, last_reap))
                time.sleep(REFRESH_INTERVAL)
    except KeyboardInterrupt:
        pass
    finally:
        conn.close()


if __name__ == "__main__":
    main()
