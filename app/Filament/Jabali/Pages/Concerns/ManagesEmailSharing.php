<?php

declare(strict_types=1);

namespace App\Filament\Jabali\Pages\Concerns;

use App\Models\Mailbox;
use App\Models\MailboxShare;
use App\Services\MailboxSharingService;
use App\Support\SafeError;
use Exception;
use Filament\Actions\Action;
use Filament\Forms\Components\Select;
use Filament\Forms\Components\TextInput;
use Filament\Notifications\Notification;
use Filament\Schemas\Components\Utilities\Get;
use Filament\Tables\Columns\TextColumn;
use Filament\Tables\Table;
use Illuminate\Database\Eloquent\Builder;
use Illuminate\Support\Facades\Auth;

trait ManagesEmailSharing
{
    protected function sharingTable(Table $table): Table
    {
        return $table
            ->query(
                MailboxShare::query()
                    ->whereHas('mailbox.emailDomain.domain', fn (Builder $q) => $q->where('user_id', Auth::id()))
                    ->with(['mailbox.emailDomain.domain', 'sharedWith.emailDomain.domain'])
            )
            ->columns([
                TextColumn::make('mailbox.email')
                    ->label(__('Owner Mailbox'))
                    ->icon('heroicon-o-envelope')
                    ->iconColor('primary')
                    ->searchable()
                    ->sortable(),
                TextColumn::make('sharedWith.email')
                    ->label(__('Share With'))
                    ->icon('heroicon-o-user')
                    ->iconColor('info')
                    ->searchable()
                    ->sortable(),
                TextColumn::make('folder')
                    ->label(__('Folder'))
                    ->searchable(),
                TextColumn::make('permission_level')
                    ->label(__('Permission Level'))
                    ->badge()
                    ->color(fn (MailboxShare $record): string => match ($record->permission_level) {
                        'Read' => 'gray',
                        'Read & Write' => 'info',
                        'Full Access' => 'warning',
                        'Admin' => 'danger',
                        default => 'gray',
                    }),
            ])
            ->recordActions([
                Action::make('editPermissions')
                    ->label(__('Edit'))
                    ->icon('heroicon-o-pencil')
                    ->color('info')
                    ->modalHeading(__('Permission Level'))
                    ->modalSubmitActionLabel(__('Save Changes'))
                    ->fillForm(fn (MailboxShare $record) => [
                        'acl_rights' => $record->acl_rights,
                    ])
                    ->form([
                        Select::make('acl_rights')
                            ->label(__('Permission Level'))
                            ->options([
                                'lrs' => __('Read'),
                                'lrswite' => __('Read & Write'),
                                'lrswitekx' => __('Full Access'),
                                'lrswitekxa' => __('Admin'),
                            ])
                            ->required(),
                    ])
                    ->action(function (MailboxShare $record, array $data): void {
                        // Verify ownership
                        if ($record->mailbox->emailDomain->domain->user_id !== Auth::id()) {
                            Notification::make()->title(__('Unauthorized'))->danger()->send();

                            return;
                        }

                        try {
                            $service = new MailboxSharingService($this->agent());
                            $service->updatePermissions($record, $data['acl_rights']);
                            Notification::make()->title(__('Permissions updated'))->success()->send();
                        } catch (Exception $e) {
                            Notification::make()->title(__('Error'))->body(SafeError::message($e))->danger()->send();
                        }
                    }),
                Action::make('revoke')
                    ->label(__('Revoke Share'))
                    ->icon('heroicon-o-trash')
                    ->color('danger')
                    ->requiresConfirmation()
                    ->modalHeading(__('Revoke Share'))
                    ->modalDescription(fn (MailboxShare $record) => __("Revoke ':email' access to :folder?", [
                        'email' => $record->sharedWith->email,
                        'folder' => $record->folder,
                    ]))
                    ->modalIcon('heroicon-o-trash')
                    ->modalIconColor('danger')
                    ->modalSubmitActionLabel(__('Revoke Share'))
                    ->action(function (MailboxShare $record): void {
                        // Verify ownership
                        if ($record->mailbox->emailDomain->domain->user_id !== Auth::id()) {
                            Notification::make()->title(__('Unauthorized'))->danger()->send();

                            return;
                        }

                        try {
                            $service = new MailboxSharingService($this->agent());
                            $service->revokeShare($record);
                            Notification::make()->title(__('Share revoked'))->success()->send();
                        } catch (Exception $e) {
                            Notification::make()->title(__('Error'))->body(SafeError::message($e))->danger()->send();
                        }
                    }),
            ])
            ->emptyStateHeading(__('No shared folders'))
            ->emptyStateDescription(__('Share mailbox folders with other users in the same domain.'))
            ->emptyStateIcon('heroicon-o-share')
            ->striped();
    }

    protected function createShareAction(): Action
    {
        return Action::make('createShare')
            ->label(__('Share Folder'))
            ->icon('heroicon-o-share')
            ->color('success')
            ->visible(fn () => $this->activeTab === 'sharing' && Mailbox::whereHas('emailDomain.domain', fn ($q) => $q->where('user_id', Auth::id()))->exists())
            ->modalHeading(__('Share Folder'))
            ->modalDescription(__('Select a folder to share'))
            ->modalIcon('heroicon-o-share')
            ->modalIconColor('success')
            ->modalSubmitActionLabel(__('Share Folder'))
            ->form([
                Select::make('mailbox_id')
                    ->label(__('Owner Mailbox'))
                    ->options(fn () => Mailbox::whereHas('emailDomain.domain', fn ($q) => $q->where('user_id', Auth::id()))
                        ->with('emailDomain.domain')
                        ->get()
                        ->mapWithKeys(fn ($m) => [$m->id => $m->email])
                        ->toArray())
                    ->required()
                    ->searchable()
                    ->live(),
                Select::make('shared_with_mailbox_id')
                    ->label(__('Share With'))
                    ->options(function (Get $get) {
                        $mailboxId = $get('mailbox_id');
                        if (! $mailboxId) {
                            return [];
                        }
                        $mailbox = Mailbox::find($mailboxId);
                        if (! $mailbox) {
                            return [];
                        }

                        return Mailbox::where('email_domain_id', $mailbox->email_domain_id)
                            ->where('id', '!=', $mailboxId)
                            ->with('emailDomain.domain')
                            ->get()
                            ->mapWithKeys(fn ($m) => [$m->id => $m->email])
                            ->toArray();
                    })
                    ->required()
                    ->searchable(),
                TextInput::make('folder')
                    ->label(__('Folder'))
                    ->default('INBOX')
                    ->required()
                    ->regex('/^(INBOX|[A-Za-z0-9._-]+)$/'),
                Select::make('acl_rights')
                    ->label(__('Permission Level'))
                    ->options([
                        'lrs' => __('Read'),
                        'lrswite' => __('Read & Write'),
                        'lrswitekx' => __('Full Access'),
                        'lrswitekxa' => __('Admin'),
                    ])
                    ->default('lrs')
                    ->required(),
            ])
            ->action(function (array $data): void {
                $owner = Mailbox::whereHas('emailDomain.domain', fn ($q) => $q->where('user_id', Auth::id()))
                    ->find($data['mailbox_id']);

                if (! $owner) {
                    Notification::make()->title(__('Mailbox not found'))->danger()->send();

                    return;
                }

                $recipient = Mailbox::where('email_domain_id', $owner->email_domain_id)
                    ->find($data['shared_with_mailbox_id']);

                if (! $recipient) {
                    Notification::make()->title(__('Mailbox not found'))->danger()->send();

                    return;
                }

                try {
                    $service = new MailboxSharingService($this->agent());
                    $service->shareFolder($owner, $recipient, $data['folder'], $data['acl_rights']);
                    Notification::make()->title(__('Folder shared successfully'))->success()->send();
                    $this->setTab('sharing');
                } catch (\InvalidArgumentException $e) {
                    Notification::make()->title(__('Error'))->body(__($e->getMessage()))->danger()->send();
                } catch (Exception $e) {
                    Notification::make()->title(__('Error'))->body(SafeError::message($e))->danger()->send();
                }
            });
    }
}
