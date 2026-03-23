<?php

declare(strict_types=1);

namespace App\Filament\Jabali\Pages\Concerns;

use App\Models\Domain;
use App\Models\EmailForwarder;
use App\Models\Mailbox;
use App\Services\RoundcubeIdentityService;
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
use Illuminate\Support\Facades\Log;

trait ManagesForwarders
{
    protected function forwardersTable(Table $table): Table
    {
        return $table
            ->query(
                EmailForwarder::query()
                    ->whereHas('emailDomain.domain', fn (Builder $q) => $q->where('user_id', Auth::id()))
                    ->with('emailDomain.domain')
            )
            ->columns([
                TextColumn::make('email')
                    ->label(__('From'))
                    ->icon('heroicon-o-arrow-right')
                    ->iconColor('primary')
                    ->searchable()
                    ->sortable(),
                TextColumn::make('destinations')
                    ->label(__('Forward To'))
                    ->badge()
                    ->separator(',')
                    ->color('gray'),
                TextColumn::make('is_active')
                    ->label(__('Status'))
                    ->badge()
                    ->formatStateUsing(fn (bool $state) => $state ? __('Active') : __('Disabled'))
                    ->color(fn (bool $state) => $state ? 'success' : 'danger'),
            ])
            ->recordActions([
                Action::make('edit')
                    ->label(__('Edit'))
                    ->icon('heroicon-o-pencil')
                    ->color('info')
                    ->modalHeading(__('Edit Forwarder'))
                    ->modalDescription(fn (EmailForwarder $record) => $record->email)
                    ->modalIcon('heroicon-o-pencil')
                    ->modalIconColor('info')
                    ->modalSubmitActionLabel(__('Save Changes'))
                    ->fillForm(fn (EmailForwarder $record) => [
                        'destinations' => implode(', ', $record->destinations ?? []),
                    ])
                    ->form([
                        TextInput::make('destinations')
                            ->label(__('Forward To'))
                            ->required()
                            ->helperText(__('Comma-separated email addresses')),
                    ])
                    ->action(fn (EmailForwarder $record, array $data) => $this->updateForwarderDirect($record, $data['destinations'])),
                Action::make('toggle')
                    ->label(fn (EmailForwarder $record) => $record->is_active ? __('Disable') : __('Enable'))
                    ->icon(fn (EmailForwarder $record) => $record->is_active ? 'heroicon-o-pause' : 'heroicon-o-play')
                    ->color('gray')
                    ->action(fn (EmailForwarder $record) => $this->toggleForwarder($record->id)),
                Action::make('delete')
                    ->label(__('Delete'))
                    ->icon('heroicon-o-trash')
                    ->color('danger')
                    ->requiresConfirmation()
                    ->modalHeading(__('Delete Forwarder'))
                    ->modalDescription(fn (EmailForwarder $record) => __("Delete forwarder ':email'?", ['email' => $record->email]))
                    ->modalIcon('heroicon-o-trash')
                    ->modalIconColor('danger')
                    ->modalSubmitActionLabel(__('Delete Forwarder'))
                    ->action(fn (EmailForwarder $record) => $this->deleteForwarderDirect($record)),
            ])
            ->emptyStateHeading(__('No forwarders yet'))
            ->emptyStateDescription(__('Create a forwarder to redirect emails to another address.'))
            ->emptyStateIcon('heroicon-o-arrow-right')
            ->striped();
    }

    protected function createForwarderAction(): Action
    {
        return Action::make('createForwarder')
            ->label(__('New Forwarder'))
            ->icon('heroicon-o-arrow-right-circle')
            ->color('info')
            ->visible(fn () => Domain::where('user_id', Auth::id())->exists())
            ->modalHeading(__('Create New Forwarder'))
            ->modalDescription(__('Redirect emails from one address to another'))
            ->modalIcon('heroicon-o-arrow-right')
            ->modalIconColor('info')
            ->modalSubmitActionLabel(__('Create Forwarder'))
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
                TextInput::make('destinations')
                    ->label(__('Forward To'))
                    ->required(fn (Get $get): bool => filled($get('domain_id')))
                    ->visible(fn (Get $get): bool => filled($get('domain_id')))
                    ->helperText(__('Comma-separated email addresses to forward to')),
            ])
            ->action(function (array $data): void {
                $domain = Domain::where('user_id', Auth::id())->find($data['domain_id']);
                if (! $domain) {
                    Notification::make()->title(__('Domain not found'))->danger()->send();

                    return;
                }

                $destinations = array_map('trim', explode(',', $data['destinations']));
                $destinations = array_filter($destinations, fn ($d) => filter_var($d, FILTER_VALIDATE_EMAIL));

                if (empty($destinations)) {
                    Notification::make()->title(__('Invalid destination emails'))->danger()->send();

                    return;
                }

                try {
                    // Get or create EmailDomain (enables email on server if needed)
                    $emailDomain = $this->getOrCreateEmailDomain($domain);

                    $email = $data['local_part'].'@'.$domain->domain;

                    if (EmailForwarder::where('email_domain_id', $emailDomain->id)->where('local_part', $data['local_part'])->exists()) {
                        Notification::make()->title(__('Forwarder already exists'))->danger()->send();

                        return;
                    }

                    if (Mailbox::where('email_domain_id', $emailDomain->id)->where('local_part', $data['local_part'])->exists()) {
                        Notification::make()->title(__('A mailbox with this address already exists'))->danger()->send();

                        return;
                    }

                    $this->agent()->send('email.forwarder_create', [
                        'username' => $this->getUsername(),
                        'email' => $email,
                        'destinations' => $destinations,
                    ]);

                    EmailForwarder::create([
                        'email_domain_id' => $emailDomain->id,
                        'user_id' => Auth::id(),
                        'local_part' => $data['local_part'],
                        'destinations' => $destinations,
                        'is_active' => true,
                    ]);

                    // Sync identities for local mailboxes in destinations
                    if (config('jabali.mail_backend') === 'stalwart') {
                        $this->syncStalwartIdentitiesForForwarder($email, $destinations);
                    } else {
                        $this->syncRoundcubeIdentitiesForForwarder($email, $destinations);
                    }

                    $this->syncMailRouting();

                    Notification::make()->title(__('Forwarder created'))->success()->send();
                } catch (Exception $e) {
                    Notification::make()->title(__('Error creating forwarder'))->body(SafeError::message($e))->danger()->send();
                }
            });
    }

    public function updateForwarderDirect(EmailForwarder $forwarder, string $destinationsString): void
    {
        $destinations = array_map('trim', explode(',', $destinationsString));
        $destinations = array_filter($destinations, fn ($d) => filter_var($d, FILTER_VALIDATE_EMAIL));

        if (empty($destinations)) {
            Notification::make()->title(__('Invalid destination emails'))->danger()->send();

            return;
        }

        try {
            $oldDestinations = $forwarder->destinations;

            $this->agent()->send('email.forwarder_update', [
                'username' => $this->getUsername(),
                'email' => $forwarder->email,
                'destinations' => $destinations,
            ]);

            $forwarder->update(['destinations' => $destinations]);

            // Update identities: remove old, add new
            if (config('jabali.mail_backend') === 'stalwart') {
                $this->removeStalwartIdentitiesForForwarder($forwarder->email, $oldDestinations);
                $this->syncStalwartIdentitiesForForwarder($forwarder->email, $destinations);
            } else {
                $this->removeRoundcubeIdentitiesForForwarder($forwarder->email, $oldDestinations);
                $this->syncRoundcubeIdentitiesForForwarder($forwarder->email, $destinations);
            }

            $this->syncMailRouting();

            Notification::make()->title(__('Forwarder updated'))->success()->send();
        } catch (Exception $e) {
            Notification::make()->title(__('Error'))->body(SafeError::message($e))->danger()->send();
        }
    }

    public function toggleForwarder(int $forwarderId): void
    {
        $forwarder = EmailForwarder::with('emailDomain.domain')->find($forwarderId);
        if (! $forwarder) {
            Notification::make()->title(__('Forwarder not found'))->danger()->send();

            return;
        }

        try {
            $newStatus = ! $forwarder->is_active;
            $this->agent()->send('email.forwarder_toggle', [
                'username' => $this->getUsername(),
                'email' => $forwarder->email,
                'active' => $newStatus,
            ]);
            $forwarder->update(['is_active' => $newStatus]);

            $this->syncMailRouting();

            Notification::make()
                ->title($newStatus ? __('Forwarder enabled') : __('Forwarder disabled'))
                ->success()
                ->send();
        } catch (Exception $e) {
            Notification::make()->title(__('Error'))->body(SafeError::message($e))->danger()->send();
        }
    }

    public function deleteForwarderDirect(EmailForwarder $forwarder): void
    {
        try {
            $forwarderEmail = $forwarder->email;
            $forwarderDestinations = $forwarder->destinations;

            $this->agent()->send('email.forwarder_delete', [
                'username' => $this->getUsername(),
                'email' => $forwarderEmail,
            ]);

            $forwarder->delete();

            // Remove identities for this forwarder
            if (config('jabali.mail_backend') === 'stalwart') {
                $this->removeStalwartIdentitiesForForwarder($forwarderEmail, $forwarderDestinations);
            } else {
                $this->removeRoundcubeIdentitiesForForwarder($forwarderEmail, $forwarderDestinations);
            }

            $this->syncMailRouting();

            Notification::make()->title(__('Forwarder deleted'))->success()->send();
        } catch (Exception $e) {
            Notification::make()->title(__('Error'))->body(SafeError::message($e))->danger()->send();
        }
    }

    /**
     * Sync Roundcube identities when a forwarder is created.
     * For each destination that is a local mailbox, add the forwarder address as an identity.
     */
    protected function syncRoundcubeIdentitiesForForwarder(string $forwarderEmail, array $destinations): void
    {
        try {
            $service = new RoundcubeIdentityService;

            foreach ($destinations as $destination) {
                // Check if destination is a local mailbox
                $mailbox = $this->findLocalMailbox($destination);
                if ($mailbox) {
                    $service->addIdentity($destination, $forwarderEmail, $mailbox->name ?? '');
                }
            }
        } catch (Exception $e) {
            Log::warning("Failed to sync Roundcube identities: {$e->getMessage()}");
        }
    }

    /**
     * Remove Roundcube identities when a forwarder is deleted.
     */
    protected function removeRoundcubeIdentitiesForForwarder(string $forwarderEmail, array $destinations): void
    {
        try {
            $service = new RoundcubeIdentityService;

            foreach ($destinations as $destination) {
                $mailbox = $this->findLocalMailbox($destination);
                if ($mailbox) {
                    $service->removeIdentity($destination, $forwarderEmail);
                }
            }
        } catch (Exception $e) {
            Log::warning("Failed to remove Roundcube identities: {$e->getMessage()}");
        }
    }

    /**
     * Sync Stalwart identities when a forwarder is created.
     * For each destination that is a local mailbox, add the forwarder address to the principal's emails.
     */
    protected function syncStalwartIdentitiesForForwarder(string $forwarderEmail, array $destinations): void
    {
        try {
            foreach ($destinations as $destination) {
                $mailbox = $this->findLocalMailbox($destination);
                if (! $mailbox) {
                    continue;
                }

                $result = $this->agent()->send('email.stalwart_add_identity', [
                    'email' => $mailbox->email,
                    'identity' => $forwarderEmail,
                    'password' => $mailbox->plain_password ?? '',
                ]);
            }
        } catch (Exception $e) {
            Log::warning("Failed to sync Stalwart identities: {$e->getMessage()}");
        }
    }

    /**
     * Remove Stalwart identities when a forwarder is deleted.
     */
    protected function removeStalwartIdentitiesForForwarder(string $forwarderEmail, array $destinations): void
    {
        try {
            foreach ($destinations as $destination) {
                $mailbox = $this->findLocalMailbox($destination);
                if (! $mailbox) {
                    continue;
                }

                $result = $this->agent()->send('email.stalwart_remove_identity', [
                    'email' => $mailbox->email,
                    'identity' => $forwarderEmail,
                    'password' => $mailbox->plain_password ?? '',
                ]);
            }
        } catch (Exception $e) {
            Log::warning("Failed to remove Stalwart identities: {$e->getMessage()}");
        }
    }

    /**
     * Find a local mailbox by email address.
     */
    protected function findLocalMailbox(string $email): ?Mailbox
    {
        $parts = explode('@', $email);
        if (count($parts) !== 2) {
            return null;
        }

        [$localPart, $domainName] = $parts;

        return Mailbox::whereHas('emailDomain.domain', function ($q) use ($domainName) {
            $q->where('domain', $domainName)->where('user_id', Auth::id());
        })->where('local_part', $localPart)->first();
    }
}
