package contracts

// Event types re-exported for API consumers.
const (
	EventTaskCreated   = "task.created"
	EventTaskStarted   = "task.started"
	EventTaskCompleted = "task.completed"
	EventTaskFailed    = "task.failed"

	EventAutomationStarted   = "automation.started"
	EventAutomationCompleted = "automation.completed"
	EventAutomationFailed    = "automation.failed"
	EventChatToken           = "chat.token"
	EventChatComplete        = "chat.complete"
)
