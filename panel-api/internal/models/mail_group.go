package models

import "time"

// MailGroup is a domain-scoped mail group that owns shared resources
// (mailbox / calendar / address book / file folder). Mailboxes are placed
// into a group via MailGroupMember; membership grants every member access
// to the group's resources in their own JMAP session (Stalwart-native
// sharing — ADR-0132, issue #201).
//
// DB-as-truth: this row is authoritative. The reconciler projects the
// group into the Stalwart registry as an @type:Group principal (which
// auto-owns a calendar + address book + its own inbox + a sending
// identity) and projects membership into each member's memberGroupIds.
//
// EmailCached mirrors the mailboxes table: a denormalised
// local_part + '@' + domains.name maintained by BEFORE INSERT/UPDATE
// triggers — not written from Go. Stalwart's queryRecipient matches it so
// inbound mail to the group address is delivered to the group inbox.
type MailGroup struct {
	ID           string `gorm:"type:char(26);primaryKey" json:"id"`
	DomainID     string `gorm:"type:char(26);not null;index:ix_mail_groups_domain" json:"domain_id"`
	LocalPart    string `gorm:"type:varchar(64);not null" json:"local_part"`
	EmailCached  string `gorm:"type:varchar(320);not null;uniqueIndex:ux_mail_groups_email_cached" json:"email"`
	DisplayName  string `gorm:"column:display_name;type:varchar(255);not null;default:''" json:"display_name"`
	Description  string `gorm:"type:varchar(255);not null;default:''" json:"description"`
	GroupKind    string `gorm:"column:group_kind;type:varchar(16);not null;default:'resource'" json:"group_kind"`
	HasMailbox   bool   `gorm:"column:has_mailbox;type:tinyint(1);not null;default:1" json:"has_mailbox"`
	HasCalendar  bool   `gorm:"column:has_calendar;type:tinyint(1);not null;default:1" json:"has_calendar"`
	HasAddressbook bool `gorm:"column:has_addressbook;type:tinyint(1);not null;default:1" json:"has_addressbook"`
	HasFiles     bool   `gorm:"column:has_files;type:tinyint(1);not null;default:1" json:"has_files"`
	CreatedAt    time.Time `gorm:"type:datetime(6);not null" json:"created_at"`
	UpdatedAt    time.Time `gorm:"type:datetime(6);not null" json:"updated_at"`
}

func (MailGroup) TableName() string { return "mail_groups" }

// MailGroupMember links a mailbox to a group. Both FKs cascade. The
// composite PK (GroupID, MailboxID) makes membership idempotent at the DB
// layer.
type MailGroupMember struct {
	GroupID   string    `gorm:"type:char(26);primaryKey" json:"group_id"`
	MailboxID string    `gorm:"type:char(26);primaryKey" json:"mailbox_id"`
	CreatedAt time.Time `gorm:"type:datetime(6);not null" json:"created_at"`
}

func (MailGroupMember) TableName() string { return "mail_group_members" }
