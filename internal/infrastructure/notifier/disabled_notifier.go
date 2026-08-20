package notifier

import "context"

type DisabledNotifier struct{}

func NewDisabledNotifier() *DisabledNotifier {
	return &DisabledNotifier{}
}

func (n *DisabledNotifier) Enabled() bool {
	return false
}

func (n *DisabledNotifier) Send(ctx context.Context, subject, body string) error {
	return nil
}
