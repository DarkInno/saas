package notification

import "errors"

var (
	ErrInvalidMessage     = errors.New("gotenancy/notification: invalid message")
	ErrInvalidSMTPConfig  = errors.New("gotenancy/notification: invalid smtp config")
	ErrUnsupportedChannel = errors.New("gotenancy/notification: unsupported channel")
	ErrTLSRequired        = errors.New("gotenancy/notification: tls required")
)
