package websocket

// RegisterExtendedHandlers adds universal agent handlers to the server
func (es *ExtendedServer) RegisterExtendedHandlers() {
	// Add universal agent handlers
	universalHandlers := map[string]MessageHandler{
		// Universal agent management
		"agent.universal.register": es.HandleUniversalAgentRegister,
		"agent.universal.discover": es.HandleUniversalAgentDiscover,
		"agent.universal.message":  es.HandleAgentMessage,
		"agent.universal.health":   es.HandleAgentHealth,

		// Backward compatibility - override existing handlers if configured
		"agent.register": es.HandleUniversalAgentRegister,
		"agent.discover": es.HandleUniversalAgentDiscover,
	}

	// Merge with existing handlers
	for name, handler := range universalHandlers {
		es.handlers[name] = handler
	}

	es.logger.Info("Registered extended universal agent handlers", map[string]interface{}{
		"handler_count": len(universalHandlers),
	})
}
