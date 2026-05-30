package mq

const (
	RequestQueue      = "matchmaking.request.queue"
	RequestRoutingKey = "matchmaking.request"

	ResultQueue      = "match.result.queue"
	ResultRoutingKey = "match.result"

	ServerCreateQueue      = "server.create.queue"
	ServerCreateRoutingKey = "server.create"

	ConfirmQueue             = "match.confirm.queue"
	ConfirmBindingRoutingKey = "match.confirm.*"
	ConfirmRequestRoutingKey = "match.confirm.request"
)
