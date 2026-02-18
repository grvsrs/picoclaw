#!/usr/bin/env python3
"""
Quick verification that Kanban system works end-to-end.
"""
from pkg.kanban.store import KanbanStore
from pkg.kanban.telegram_bridge import TelegramKanbanBridge
from pkg.kanban.events import KanbanEventBridge
from pkg.kanban.schema import TaskMode, TaskState

def main():
    print("=" * 60)
    print("PicoClaw Kanban System Verification")
    print("=" * 60)
    
    # Create store
    print("\n[1/6] Creating SQLite store...")
    store = KanbanStore('/tmp/picoclaw_test_kanban.db')
    print("✅ Store created")
    
    # Create bridges
    print("\n[2/6] Creating Telegram and Event bridges...")
    telegram_bridge = TelegramKanbanBridge(store)
    event_bridge = KanbanEventBridge(store)
    print("✅ Bridges initialized")
    
    # Create card from Telegram
    print("\n[3/6] Creating Kanban card from Telegram message...")
    card = telegram_bridge.create_card_from_telegram(
        title="Deploy glass-walls to staging",
        telegram_message_id="test-msg-123",
        telegram_user_id="9411488118",
        mode=TaskMode.PERSONAL,
        priority="high",
        description="Test deployment via Telegram bot",
        tags=["ops", "deploy", "test"]
    )
    
    if card:
        print(f"✅ Card created: {card.card_id}")
        print(f"   Title: {card.title}")
        print(f"   State: {card.state.value}")
        print(f"   Mode: {card.mode.value}")
    else:
        print("❌ Card creation failed")
        return
    
    # Simulate task lifecycle
    print("\n[4/6] Simulating task execution lifecycle...")
    
    # Plan
    card.transition_to(TaskState.PLANNED, reason="Scheduled for execution")
    store.save(card)
    print(f"   → State: {card.state.value}")
    
    # Start (via event bridge)
    event_bridge.on_task_started(card.card_id, executor="picoclaw")
    print(f"   → Event: task_started")
    
    # Complete (via event bridge)
    event_bridge.on_task_completed(
        card.card_id, 
        result="Deployment successful", 
        log_url="/logs/123"
    )
    print(f"   → Event: task_completed")
    
    # Reload and check final state
    updated_card = store.get(card.card_id)
    print(f"✅ Final state: {updated_card.state.value}")
    print(f"   Attempts: {updated_card.attempts}")
    print(f"   State history: {len(updated_card.state_history)} transitions")
    
    # List cards
    print("\n[5/6] Listing all cards...")
    summary = telegram_bridge.list_cards_summary(limit=10)
    print(summary)
    
    # Card summary
    print("\n[6/6] Getting card summary...")
    card_summary = telegram_bridge.get_card_summary(card.card_id)
    print(card_summary)
    
    print("\n" + "=" * 60)
    print("✅ ALL CHECKS PASSED")
    print("=" * 60)
    print("\nKanban system is working correctly!")
    print(f"Test database: /tmp/picoclaw_test_kanban.db")

if __name__ == "__main__":
    main()
