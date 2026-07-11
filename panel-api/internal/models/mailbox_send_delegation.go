package models

import "time"

// MailboxSendDelegation grants a delegate mailbox permission to send email FROM a
// grantor mailbox's address, WITHOUT the delegate receiving the grantor's mail
// (GH #347). Jabali is truth (ADR-0042); the converger materialises the current
// set of pairs into Stalwart's MtaStageAuth.mustMatchSender expression
// (Mechanism C — project_jab347_spike_verdict). queryRecipient is untouched, so
// the grantor keeps receiving its own mail.
//
// Column tags are pinned explicitly (gorm_column_tags scar: GORM would otherwise
// split the *MailboxID initialisms in surprising ways).
type MailboxSendDelegation struct {
	ID                string    `gorm:"column:id;type:char(26);primaryKey" json:"id"`
	DelegateMailboxID string    `gorm:"column:delegate_mailbox_id;type:char(26);not null;uniqueIndex:uq_mailbox_send_deleg,priority:1;index:ix_msd_delegate" json:"delegate_mailbox_id"`
	GrantorMailboxID  string    `gorm:"column:grantor_mailbox_id;type:char(26);not null;uniqueIndex:uq_mailbox_send_deleg,priority:2;index:ix_msd_grantor" json:"grantor_mailbox_id"`
	CreatedAt         time.Time `gorm:"column:created_at;type:datetime(6);not null" json:"created_at"`
	UpdatedAt         time.Time `gorm:"column:updated_at;type:datetime(6);not null" json:"updated_at"`
}

func (MailboxSendDelegation) TableName() string { return "mailbox_send_delegations" }

// SendAsPair is a resolved (delegate email, grantor email) pair used to build the
// Stalwart mustMatchSender expression. Emails are the mailboxes' email_cached.
type SendAsPair struct {
	DelegateEmail string `json:"delegate_email"`
	GrantorEmail  string `json:"grantor_email"`
}
