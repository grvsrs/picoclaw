# Kanban system: task tracking, state management, and event-driven updates
# 
# Components:
#   schema.py      - Data model (KanbanCard, TaskState, TaskCategory, TaskSource)
#   store.py       - SQLite persistence layer
#   events.py      - Event bridge for executor integration
#   categorizer.py - LLM-powered and rule-based task categorization
#   mode.py        - Execution mode management (personal/remote)
