package notification

import "errors"

var (
	ErrInvalidMessage        = errors.New("saas/notification: invalid message")
	ErrInvalidNotifierConfig = errors.New("saas/notification: invalid notifier config")
	ErrInvalidResendConfig   = errors.New("saas/notification: invalid resend config")
	ErrInvalidSESConfig      = errors.New("saas/notification: invalid ses config")
	ErrInvalidSMTPConfig     = errors.New("saas/notification: invalid smtp config")
	ErrInvalidWebhookConfig  = errors.New("saas/notification: invalid webhook config")
	ErrNilNotifier           = errors.New("saas/notification: nil notifier")
	ErrResendDelivery        = errors.New("saas/notification: resend delivery failed")
	ErrSESDelivery           = errors.New("saas/notification: ses delivery failed")
	ErrUnsupportedChannel    = errors.New("saas/notification: unsupported channel")
	ErrWebhookDelivery       = errors.New("saas/notification: webhook delivery failed")
	ErrTLSRequired           = errors.New("saas/notification: tls required")
)
