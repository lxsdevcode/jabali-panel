<?php

declare(strict_types=1);

namespace App\Filament\Jabali\Pages\Concerns;

use App\Models\AuditLog;
use App\Models\DnsSetting;
use App\Models\Domain;
use App\Models\Mailbox;
use App\Support\SafeError;
use App\Support\WordList;
use Exception;
use Filament\Actions\Action;
use Filament\Forms\Components\Select;
use Filament\Forms\Components\TextInput;
use Filament\Forms\Components\Toggle;
use Filament\Infolists\Components\TextEntry;
use Filament\Notifications\Notification;
use Filament\Schemas\Components\Section;
use Filament\Schemas\Components\Utilities\Get;
use Filament\Tables\Columns\TextColumn;
use Filament\Tables\Table;
use Illuminate\Database\Eloquent\Builder;
use Illuminate\Support\Facades\Auth;
use Illuminate\Support\Facades\Crypt;

trait ManagesMailboxes
{
    public string $credEmail = '';

    public string $credPassword = '';

    protected function mailboxesTable(Table $table): Table
    {
        return $table
            ->query(
                Mailbox::query()
                    ->whereHas('emailDomain.domain', fn (Builder $q) => $q->where('user_id', Auth::id()))
                    ->with('emailDomain.domain')
            )
            ->columns([
                TextColumn::make('email')
                    ->label(__('Email Address'))
                    ->icon('heroicon-o-envelope')
                    ->iconColor('primary')
                    ->description(fn (Mailbox $record) => $record->name)
                    ->searchable()
                    ->sortable(),
                TextColumn::make('quota_display')
                    ->label(__('Quota'))
                    ->getStateUsing(fn (Mailbox $record) => $record->quota_used_formatted.' / '.$record->quota_formatted)
                    ->description(fn (Mailbox $record) => $record->quota_percent.'% '.__('used'))
                    ->color(fn (Mailbox $record) => match (true) {
                        $record->quota_percent >= 90 => 'danger',
                        $record->quota_percent >= 80 => 'warning',
                        default => 'gray',
                    }),
                TextColumn::make('is_active')
                    ->label(__('Status'))
                    ->badge()
                    ->formatStateUsing(fn (bool $state) => $state ? __('Active') : __('Suspended'))
                    ->color(fn (bool $state) => $state ? 'success' : 'danger'),
                TextColumn::make('last_login_at')
                    ->label(__('Last Login'))
                    ->since()
                    ->placeholder(__('Never'))
                    ->sortable(),
            ])
            ->recordActions([
                Action::make('webmail')
                    ->label(__('Webmail'))
                    ->icon('heroicon-o-envelope-open')
                    ->color('success')
                    ->url(fn (Mailbox $record) => route('webmail.sso', $record))
                    ->openUrlInNewTab(),
                Action::make('info')
                    ->label(__('Info'))
                    ->icon('heroicon-o-information-circle')
                    ->color('info')
                    ->modalHeading(fn (Mailbox $record) => __('Connection Settings'))
                    ->modalDescription(fn (Mailbox $record) => $record->email)
                    ->modalSubmitAction(false)
                    ->modalCancelActionLabel(__('Close'))
                    ->infolist(function (Mailbox $record): array {
                        $domain = $record->emailDomain?->domain?->domain ?? '';
                        $serverHostname = $domain ? "mail.{$domain}" : (\App\Models\Setting::get('mail_hostname') ?: request()->getHost());

                        return [
                            Section::make(__('IMAP Settings'))
                                ->description(__('For receiving email in mail clients'))
                                ->icon('heroicon-o-inbox-arrow-down')
                                ->columns(3)
                                ->schema([
                                    TextEntry::make('imap_server')
                                        ->label(__('Server'))
                                        ->state($serverHostname)
                                        ->copyable(),
                                    TextEntry::make('imap_port')
                                        ->label(__('Port'))
                                        ->state('993')
                                        ->copyable(),
                                    TextEntry::make('imap_security')
                                        ->label(__('Security'))
                                        ->state('SSL/TLS')
                                        ->badge()
                                        ->color('success'),
                                ]),
                            Section::make(__('POP3 Settings'))
                                ->description(__('Alternative for receiving email'))
                                ->icon('heroicon-o-arrow-down-tray')
                                ->columns(3)
                                ->collapsed()
                                ->schema([
                                    TextEntry::make('pop3_server')
                                        ->label(__('Server'))
                                        ->state($serverHostname)
                                        ->copyable(),
                                    TextEntry::make('pop3_port')
                                        ->label(__('Port'))
                                        ->state('995')
                                        ->copyable(),
                                    TextEntry::make('pop3_security')
                                        ->label(__('Security'))
                                        ->state('SSL/TLS')
                                        ->badge()
                                        ->color('success'),
                                ]),
                            Section::make(__('SMTP Settings'))
                                ->description(__('For sending email'))
                                ->icon('heroicon-o-paper-airplane')
                                ->columns(3)
                                ->schema([
                                    TextEntry::make('smtp_server')
                                        ->label(__('Server'))
                                        ->state($serverHostname)
                                        ->copyable(),
                                    TextEntry::make('smtp_port')
                                        ->label(__('Port'))
                                        ->state('465')
                                        ->copyable(),
                                    TextEntry::make('smtp_security')
                                        ->label(__('Security'))
                                        ->state('SSL/TLS')
                                        ->badge()
                                        ->color('success'),
                                ]),
                            Section::make(__('Credentials'))
                                ->description(__('Use your email address and password'))
                                ->icon('heroicon-o-key')
                                ->columns(2)
                                ->schema([
                                    TextEntry::make('username')
                                        ->label(__('Username'))
                                        ->state($record->email)
                                        ->copyable(),
                                    TextEntry::make('password_hint')
                                        ->label(__('Password'))
                                        ->state(__('Your mailbox password')),
                                ]),
                        ];
                    }),
                Action::make('password')
                    ->label(__('Password'))
                    ->icon('heroicon-o-key')
                    ->color('warning')
                    ->modalHeading(__('Change Password'))
                    ->modalDescription(fn (Mailbox $record) => $record->email)
                    ->modalIcon('heroicon-o-key')
                    ->modalIconColor('warning')
                    ->modalSubmitActionLabel(__('Change Password'))
                    ->form([
                        TextInput::make('password')
                            ->label(__('New Password'))
                            ->password()
                            ->revealable()
                            ->required()
                            ->minLength(8)
                            ->rules(fn () => (bool) DnsSetting::get('passphrase_passwords') ? [] : [
                                'regex:/[a-z]/',
                                'regex:/[A-Z]/',
                                'regex:/[0-9]/',
                            ])
                            ->default(fn () => (bool) DnsSetting::get('passphrase_passwords') ? WordList::generate() : $this->generateSecurePassword())
                            ->suffixActions([
                                Action::make('generatePassword')
                                    ->icon('heroicon-o-arrow-path')
                                    ->tooltip(__('Generate secure password'))
                                    ->action(function ($set) {
                                        $password = (bool) DnsSetting::get('passphrase_passwords') ? WordList::generate() : $this->generateSecurePassword();
                                        $set('password', $password);
                                    }),
                                Action::make('copyPassword')
                                    ->icon('heroicon-o-clipboard-document')
                                    ->tooltip(__('Copy to clipboard'))
                                    ->action(function ($state, $livewire) {
                                        if ($state) {
                                            $livewire->js('navigator.clipboard.writeText('.json_encode($state, JSON_HEX_TAG).')');
                                            Notification::make()
                                                ->title(__('Copied to clipboard'))
                                                ->success()
                                                ->duration(2000)
                                                ->send();
                                        }
                                    }),
                            ])
                            ->helperText(fn () => (bool) DnsSetting::get('passphrase_passwords')
                                ? __('Password will be generated as easy-to-remember words')
                                : __('Minimum 8 characters with uppercase, lowercase, and numbers')),
                    ])
                    ->action(fn (Mailbox $record, array $data) => $this->changeMailboxPasswordDirect($record, $data['password'])),
                Action::make('toggle')
                    ->label(fn (Mailbox $record) => $record->is_active ? __('Suspend') : __('Enable'))
                    ->icon(fn (Mailbox $record) => $record->is_active ? 'heroicon-o-pause' : 'heroicon-o-play')
                    ->color('gray')
                    ->action(fn (Mailbox $record) => $this->toggleMailbox($record->id)),
                Action::make('delete')
                    ->label(__('Delete'))
                    ->icon('heroicon-o-trash')
                    ->color('danger')
                    ->requiresConfirmation()
                    ->modalHeading(__('Delete Mailbox'))
                    ->modalDescription(fn (Mailbox $record) => __("Delete ':email'? All emails will be lost.", ['email' => $record->email]))
                    ->modalIcon('heroicon-o-trash')
                    ->modalIconColor('danger')
                    ->modalSubmitActionLabel(__('Delete Mailbox'))
                    ->form([
                        Toggle::make('delete_files')
                            ->label(__('Also delete all email files'))
                            ->default(false)
                            ->helperText(__('Warning: This cannot be undone')),
                    ])
                    ->action(fn (Mailbox $record, array $data) => $this->deleteMailboxDirect($record, $data['delete_files'] ?? false)),
            ])
            ->emptyStateHeading(__('No mailboxes yet'))
            ->emptyStateDescription(__('Enable email for a domain first, then create a mailbox.'))
            ->emptyStateIcon('heroicon-o-envelope')
            ->striped();
    }

    protected function createMailboxAction(): Action
    {
        return Action::make('createMailbox')
            ->label(__('New Mailbox'))
            ->icon('heroicon-o-plus-circle')
            ->color('success')
            ->visible(fn () => Domain::where('user_id', Auth::id())->exists())
            ->modalHeading(__('Create New Mailbox'))
            ->modalDescription(__('Create an email account for one of your domains'))
            ->modalIcon('heroicon-o-envelope')
            ->modalIconColor('success')
            ->modalSubmitActionLabel(__('Create Mailbox'))
            ->form([
                Select::make('domain_id')
                    ->label(__('Domain'))
                    ->options(fn () => Domain::where('user_id', Auth::id())->pluck('domain', 'id')->toArray())
                    ->required()
                    ->searchable()
                    ->live(),
                TextInput::make('local_part')
                    ->label(__('Email Address'))
                    ->required(fn (Get $get): bool => filled($get('domain_id')))
                    ->visible(fn (Get $get): bool => filled($get('domain_id')))
                    ->regex('/^[a-zA-Z0-9._%+-]+$/')
                    ->maxLength(64)
                    ->helperText(__('The part before the @ symbol')),
                TextInput::make('name')
                    ->label(__('Display Name'))
                    ->visible(fn (Get $get): bool => filled($get('domain_id')))
                    ->maxLength(255),
                TextInput::make('password')
                    ->label(__('Password'))
                    ->password()
                    ->revealable()
                    ->required(fn (Get $get): bool => filled($get('domain_id')))
                    ->visible(fn (Get $get): bool => filled($get('domain_id')))
                    ->minLength(8)
                    ->rules(fn () => (bool) DnsSetting::get('passphrase_passwords') ? [] : [
                        'regex:/[a-z]/',
                        'regex:/[A-Z]/',
                        'regex:/[0-9]/',
                    ])
                    ->default(fn () => (bool) DnsSetting::get('passphrase_passwords') ? WordList::generate() : $this->generateSecurePassword())
                    ->suffixActions([
                        Action::make('generatePassword')
                            ->icon('heroicon-o-arrow-path')
                            ->tooltip(__('Generate secure password'))
                            ->action(function ($set) {
                                $password = (bool) DnsSetting::get('passphrase_passwords') ? WordList::generate() : $this->generateSecurePassword();
                                $set('password', $password);
                            }),
                        Action::make('copyPassword')
                            ->icon('heroicon-o-clipboard-document')
                            ->tooltip(__('Copy to clipboard'))
                            ->action(function ($state, $livewire) {
                                if ($state) {
                                    $livewire->js('navigator.clipboard.writeText('.json_encode($state, JSON_HEX_TAG).')');
                                    Notification::make()
                                        ->title(__('Copied to clipboard'))
                                        ->success()
                                        ->duration(2000)
                                        ->send();
                                }
                            }),
                    ])
                    ->helperText(fn () => (bool) DnsSetting::get('passphrase_passwords')
                        ? __('Password will be generated as easy-to-remember words')
                        : __('Minimum 8 characters with uppercase, lowercase, and numbers')),
                TextInput::make('quota_mb')
                    ->label(__('Quota (MB)'))
                    ->numeric()
                    ->visible(fn (Get $get): bool => filled($get('domain_id')))
                    ->default(1024)
                    ->minValue(100)
                    ->maxValue(10240)
                    ->helperText(__('Storage limit in megabytes')),
            ])
            ->action(function (array $data): void {
                $limit = Auth::user()?->hostingPackage?->mailboxes_limit;
                if ($limit && Mailbox::where('user_id', Auth::id())->count() >= $limit) {
                    Notification::make()
                        ->title(__('Mailbox limit reached'))
                        ->body(__('Your hosting package allows up to :limit mailboxes.', ['limit' => $limit]))
                        ->warning()
                        ->send();

                    return;
                }

                $domain = Domain::where('user_id', Auth::id())->find($data['domain_id']);
                if (! $domain) {
                    Notification::make()->title(__('Domain not found'))->danger()->send();

                    return;
                }

                try {
                    // Get or create EmailDomain (enables email on server if needed)
                    $emailDomain = $this->getOrCreateEmailDomain($domain);

                    $email = $data['local_part'].'@'.$domain->domain;
                    $quotaBytes = (int) $data['quota_mb'] * 1024 * 1024;

                    if (Mailbox::where('email_domain_id', $emailDomain->id)->where('local_part', $data['local_part'])->exists()) {
                        Notification::make()->title(__('Mailbox already exists'))->danger()->send();

                        return;
                    }

                    $result = $this->agent()->mailboxCreate(
                        $this->getUsername(),
                        $email,
                        $data['password'],
                        $quotaBytes
                    );

                    Mailbox::create([
                        'email_domain_id' => $emailDomain->id,
                        'user_id' => Auth::id(),
                        'local_part' => $data['local_part'],
                        'password_hash' => $result['password_hash'] ?? '',
                        'password_encrypted' => Crypt::encryptString($data['password']),
                        'maildir_path' => $result['maildir_path'] ?? null,
                        'system_uid' => $result['uid'] ?? null,
                        'system_gid' => $result['gid'] ?? null,
                        'name' => $data['name'],
                        'quota_bytes' => $quotaBytes,
                        'is_active' => true,
                    ]);

                    AuditLog::logEmailAction('created', $email, [
                        'domain' => $domain->domain,
                        'quota_bytes' => $quotaBytes,
                    ]);

                    $this->syncMailRouting();

                    $this->credEmail = $email;
                    $this->credPassword = $data['password'];

                    Notification::make()->title(__('Mailbox created'))->success()->send();

                    $this->mountAction('showCredentials');
                } catch (Exception $e) {
                    Notification::make()->title(__('Error creating mailbox'))->body(SafeError::message($e))->danger()->send();
                }
            });
    }

    public function changeMailboxPasswordDirect(Mailbox $mailbox, string $password): void
    {
        try {
            $result = $this->agent()->mailboxChangePassword(
                $this->getUsername(),
                $mailbox->email,
                $password
            );

            $mailbox->update([
                'password_hash' => $result['password_hash'] ?? '',
                'password_encrypted' => Crypt::encryptString($password),
            ]);

            $this->credEmail = $mailbox->email;
            $this->credPassword = $password;

            AuditLog::logEmailAction('password_changed', $mailbox->email);

            Notification::make()->title(__('Password changed'))->success()->send();

            $this->mountAction('showCredentials');
        } catch (Exception $e) {
            Notification::make()->title(__('Error'))->body(SafeError::message($e))->danger()->send();
        }
    }

    public function toggleMailbox(int $mailboxId): void
    {
        $mailbox = Mailbox::whereHas('emailDomain.domain', fn ($q) => $q->where('user_id', Auth::id()))->with('emailDomain.domain')->find($mailboxId);
        if (! $mailbox) {
            Notification::make()->title(__('Mailbox not found'))->danger()->send();

            return;
        }

        try {
            $newStatus = ! $mailbox->is_active;
            $this->agent()->mailboxToggle($this->getUsername(), $mailbox->email, $newStatus);
            $mailbox->update(['is_active' => $newStatus]);

            $this->syncMailRouting();

            AuditLog::logEmailAction($newStatus ? 'enabled' : 'disabled', $mailbox->email);

            Notification::make()
                ->title($newStatus ? __('Mailbox enabled') : __('Mailbox disabled'))
                ->success()
                ->send();
        } catch (Exception $e) {
            Notification::make()->title(__('Error'))->body(SafeError::message($e))->danger()->send();
        }
    }

    public function deleteMailboxDirect(Mailbox $mailbox, bool $deleteFiles): void
    {
        try {
            $this->agent()->mailboxDelete(
                $this->getUsername(),
                $mailbox->email,
                $deleteFiles,
                $mailbox->maildir_path
            );

            $mailbox->delete();

            $this->syncMailRouting();

            AuditLog::logEmailAction('deleted', $mailbox->email, [
                'delete_files' => $deleteFiles,
            ]);

            Notification::make()->title(__('Mailbox deleted'))->success()->send();
        } catch (Exception $e) {
            Notification::make()->title(__('Error'))->body(SafeError::message($e))->danger()->send();
        }
    }

    protected function showCredentialsAction(): Action
    {
        return Action::make('showCredentials')
            ->label(__('Credentials'))
            ->hidden()
            ->modalHeading(__('Mailbox Credentials'))
            ->modalDescription(__('Save these credentials! The password won\'t be shown again.'))
            ->modalIcon('heroicon-o-check-circle')
            ->modalIconColor('success')
            ->modalSubmitAction(false)
            ->modalCancelActionLabel(__('Done'))
            ->infolist([
                Section::make(__('Email Address'))
                    ->schema([
                        TextEntry::make('email')
                            ->hiddenLabel()
                            ->state(fn () => $this->credEmail)
                            ->copyable()
                            ->fontFamily('mono'),
                    ]),
                Section::make(__('Password'))
                    ->schema([
                        TextEntry::make('password')
                            ->hiddenLabel()
                            ->state(fn () => $this->credPassword)
                            ->copyable()
                            ->fontFamily('mono'),
                    ]),
            ]);
    }
}
