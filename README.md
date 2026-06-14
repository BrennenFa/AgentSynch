# AgentSynch

1. keep any comments that already exist

programming language: Go

Format: 
1. long term --> brew package
2. short term --> just local

database
1. short term --> yaml/json
2. medium term --> sqlite
3. long term --> tbd


goals:
1. have claude auto write different goals given by the user
2. store goals
3. have claude isntances be able to look at,  claim, and collaborate on goals



diagram from claude
You own all the infrastructure:

├── Agent registry (who's alive, what role)
├── Goal DAG (storage, dependency resolution)  
├── Blackboard (versioned key-value store)
├── Event bus (agent-to-agent messaging)
├── Claim/lock logic (prevent two agents taking same goal)
├── Heartbeat loop (detect dead agents, re-queue their work)
├── CLI / daemon process
└── SQLite schema + migrations


  go run main.go add "my first task" "description of what to do"



Interesting elements
1. concurrency
2. DAGs
3. Server + heartbeat
4. mutliagent (todo)