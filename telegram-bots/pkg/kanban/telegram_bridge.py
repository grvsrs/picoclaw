"""
Telegram to Kanban integration: convert Telegram task messages into Kanban cards.

When a bot handler executes a command, it creates a Kanban card to track the task.
"""
from datetime import datetime
from typing import Optional
from .schema import KanbanCard, TaskMode, TaskState
from .store import KanbanStore


class TelegramKanbanBridge:
    """Create and link Kanban cards from Telegram bot interactions."""
    
    def __init__(self, store: KanbanStore):
        """Initialize with a Kanban store."""
        self.store = store
    
    def create_card_from_telegram(
        self,
        title: str,
        telegram_message_id: str,
        telegram_user_id: str,
        mode: TaskMode = TaskMode.PERSONAL,
        executor: str = "picoclaw",
        priority: str = "normal",
        description: str = "",
        tags: Optional[list] = None,
    ) -> Optional[KanbanCard]:
        """
        Create a Kanban card from a Telegram task command.
        
        Returns the created card, or None if creation failed.
        """
        # Persistent card ID from DB count (survives restarts)
        card_id = self.store.next_card_id()
        
        card = KanbanCard(
            card_id=card_id,
            title=title,
            description=description or "",
            mode=mode,
            executor=executor,
            priority=priority,
            state=TaskState.INBOX,
            telegram_message_id=telegram_message_id,
            allowed_users=[telegram_user_id],
            created_by=telegram_user_id,
            tags=tags or [],
        )
        
        if self.store.save(card):
            print(f"[TELEGRAM] Created card {card_id} from message {telegram_message_id}")
            return card
        
        return None
    
    def transition_card_from_telegram(
        self,
        card_id: str,
        new_state: TaskState,
        telegram_user_id: str,
        reason: str = "",
    ) -> bool:
        """Transition a card based on Telegram interaction."""
        card = self.store.get(card_id)
        if not card:
            return False
        
        if card.transition_to(new_state, reason=reason, executor=f"telegram:{telegram_user_id}"):
            return self.store.save(card)
        
        return False
    
    def get_card_summary(self, card_id: str) -> Optional[str]:
        """Format a card as a concise summary for Telegram."""
        card = self.store.get(card_id)
        if not card:
            return None
        
        lines = [
            f"🎯 {card.card_id}: {card.title}",
            f"📊 State: {card.state.value}",
            f"⚙️ Mode: {card.mode.value}",
            f"🎪 Attempts: {card.attempts}",
        ]
        
        if card.priority != "normal":
            lines.append(f"⚡ Priority: {card.priority}")
        
        if card.last_failure_reason:
            lines.append(f"❌ Reason: {card.last_failure_reason}")
        
        if card.execution_log_url:
            lines.append(f"📝 Logs: {card.execution_log_url}")
        
        return "\n".join(lines)
    
    def list_cards_summary(self, state: Optional[TaskState] = None, limit: int = 10) -> str:
        """Format a list of cards for Telegram."""
        if state:
            cards = self.store.list_by_state(state, limit=limit)
        else:
            cards = self.store.list_all(limit=limit)
        
        if not cards:
            return "No cards found."
        
        lines = [f"📋 Kanban ({len(cards)} cards):"]
        for card in cards:
            state_emoji = {
                TaskState.INBOX: "📬",
                TaskState.PLANNED: "📅",
                TaskState.RUNNING: "🚀",
                TaskState.BLOCKED: "🛑",
                TaskState.REVIEW: "👀",
                TaskState.DONE: "✅",
            }.get(card.state, "❓")
            lines.append(f"{state_emoji} {card.card_id}: {card.title}")
        
        return "\n".join(lines)
