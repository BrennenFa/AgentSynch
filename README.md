## Installation

```bash
brew tap BrennenFa/AgentSynch https://github.com/BrennenFa/AgentSynch
brew install agentsynch
```

Or build from source:
```bash
cd GoCLI && make build
```

## Tip: tmux mouse scrolling
If you're navigating AgentSynch's worker tmux panes, enable mouse support for scrolling by adding to `~/.tmux.conf`:

```
set -g mouse on
```

Then reload with `tmux source-file ~/.tmux.conf`.

To start:
./agentsynch tui --> opens TUI

To start workers: 
./agentsynch worker

To add tasks:
Ask Claude


Note - Documentation is being updated

TODO
1. obsidian integration
2. MCP integration for various platforms
3. unit tests
4. subagents - do at each stage
5. dag
6. tui fixes --- add in claude status checker ( waiing for a message or now)
