<?php

declare(strict_types=1);

namespace App\Filament\Jabali\Pages\Concerns;

use App\Models\EmailDomain;
use App\Models\Mailbox;
use App\Models\UserSetting;
use App\Support\SafeError;
use Exception;
use Filament\Actions\Action;
use Filament\Forms\Components\Select;
use Filament\Forms\Components\Textarea;
use Filament\Forms\Components\TextInput;
use Filament\Forms\Components\Toggle;
use Filament\Notifications\Notification;
use Filament\Schemas\Components\Section;
use Filament\Schemas\Schema;
use Filament\Tables\Columns\TextColumn;
use Filament\Tables\Table;
use Illuminate\Database\Eloquent\Builder;
use Illuminate\Support\Facades\Auth;

trait ManagesEmailSettings
{
    public array $spamFormData = [];

    protected function catchAllTable(Table $table): Table
    {
        return $table
            ->query(
                EmailDomain::query()
                    ->whereHas('domain', fn (Builder $q) => $q->where('user_id', Auth::id()))
                    ->with('domain')
            )
            ->columns([
                TextColumn::make('domain.domain')
                    ->label(__('Domain'))
                    ->icon('heroicon-o-globe-alt')
                    ->iconColor('primary')
                    ->searchable()
                    ->sortable(),
                TextColumn::make('catch_all_enabled')
                    ->label(__('Status'))
                    ->badge()
                    ->formatStateUsing(fn (bool $state) => $state ? __('Enabled') : __('Disabled'))
                    ->color(fn (bool $state) => $state ? 'success' : 'gray'),
                TextColumn::make('catch_all_address')
                    ->label(__('Forward To'))
                    ->placeholder(__('Not configured'))
                    ->icon('heroicon-o-envelope')
                    ->iconColor('info'),
            ])
            ->recordActions([
                Action::make('configure')
                    ->label(__('Configure'))
                    ->icon('heroicon-o-cog-6-tooth')
                    ->color('info')
                    ->modalHeading(__('Configure Catch-All'))
                    ->modalDescription(fn (EmailDomain $record) => $record->domain->domain)
                    ->modalIcon('heroicon-o-inbox-stack')
                    ->modalIconColor('info')
                    ->modalSubmitActionLabel(__('Save'))
                    ->fillForm(fn (EmailDomain $record) => [
                        'enabled' => $record->catch_all_enabled,
                        'address' => $record->catch_all_address,
                    ])
                    ->form([
                        Toggle::make('enabled')
                            ->label(__('Enable Catch-All'))
                            ->helperText(__('Receive emails sent to any non-existent address on this domain')),
                        Select::make('address')
                            ->label(__('Deliver To'))
                            ->options(function (EmailDomain $record) {
                                return Mailbox::where('email_domain_id', $record->id)
                                    ->pluck('local_part')
                                    ->mapWithKeys(fn ($local) => [
                                        $local.'@'.$record->domain->domain => $local.'@'.$record->domain->domain,
                                    ])
                                    ->toArray();
                            })
                            ->searchable()
                            ->helperText(__('Select a mailbox to receive catch-all emails')),
                    ])
                    ->action(fn (EmailDomain $record, array $data) => $this->updateCatchAll($record, $data)),
            ])
            ->emptyStateHeading(__('No email domains'))
            ->emptyStateDescription(__('Create a mailbox first to enable email for a domain.'))
            ->emptyStateIcon('heroicon-o-inbox-stack')
            ->striped();
    }

    protected function disclaimerTable(Table $table): Table
    {
        return $table
            ->query(
                EmailDomain::query()
                    ->whereHas('domain', fn (Builder $q) => $q->where('user_id', Auth::id()))
                    ->with('domain')
            )
            ->columns([
                TextColumn::make('domain.domain')
                    ->label(__('Domain'))
                    ->icon('heroicon-o-globe-alt')
                    ->iconColor('primary')
                    ->searchable()
                    ->sortable(),
                TextColumn::make('disclaimer_enabled')
                    ->label(__('Status'))
                    ->badge()
                    ->formatStateUsing(fn (bool $state) => $state ? __('Enabled') : __('Disabled'))
                    ->color(fn (bool $state) => $state ? 'success' : 'gray'),
            ])
            ->recordActions([
                Action::make('configure')
                    ->label(__('Configure'))
                    ->icon('heroicon-o-cog-6-tooth')
                    ->color('info')
                    ->modalHeading(__('Email Disclaimer'))
                    ->modalDescription(fn (EmailDomain $record) => $record->domain->domain)
                    ->modalIcon('heroicon-o-document-text')
                    ->modalIconColor('info')
                    ->modalSubmitActionLabel(__('Save'))
                    ->fillForm(fn (EmailDomain $record) => [
                        'enabled' => $record->disclaimer_enabled,
                        'text' => $record->disclaimer_text ?? __('If you received this email by mistake, please notify the sender and delete it.'),
                    ])
                    ->form([
                        Toggle::make('enabled')
                            ->label(__('Enable Disclaimer'))
                            ->helperText(__('Append a disclaimer to all outbound emails from this domain')),
                        Textarea::make('text')
                            ->label(__('Disclaimer Text'))
                            ->rows(4)
                            ->helperText(__('This text will be appended to every outgoing email'))
                            ->required(),
                    ])
                    ->action(fn (EmailDomain $record, array $data) => $this->updateDisclaimer($record, $data)),
            ])
            ->emptyStateHeading(__('No email domains'))
            ->emptyStateDescription(__('Create a mailbox first to enable email for a domain.'))
            ->emptyStateIcon('heroicon-o-document-text')
            ->striped();
    }

    public function updateCatchAll(EmailDomain $emailDomain, array $data): void
    {
        try {
            $enabled = $data['enabled'] ?? false;
            $address = $data['address'] ?? null;

            if ($enabled && empty($address)) {
                Notification::make()
                    ->title(__('Error'))
                    ->body(__('Please select a mailbox to receive catch-all emails'))
                    ->danger()
                    ->send();

                return;
            }

            // Update in Postfix virtual alias maps
            $this->agent()->send('email.catchall_update', [
                'username' => $this->getUsername(),
                'domain' => $emailDomain->domain->domain,
                'enabled' => $enabled,
                'address' => $address,
            ]);

            $emailDomain->update([
                'catch_all_enabled' => $enabled,
                'catch_all_address' => $enabled ? $address : null,
            ]);

            $this->syncMailRouting();

            Notification::make()
                ->title($enabled ? __('Catch-all enabled') : __('Catch-all disabled'))
                ->success()
                ->send();
        } catch (Exception $e) {
            Notification::make()->title(__('Error'))->body(SafeError::message($e))->danger()->send();
        }
    }

    public function updateDisclaimer(EmailDomain $emailDomain, array $data): void
    {
        try {
            $enabled = $data['enabled'] ?? false;
            $text = $data['text'] ?? '';

            $this->agent()->send('email.disclaimer_update', [
                'domain' => $emailDomain->domain->domain,
                'enabled' => $enabled,
                'text' => $text,
            ]);

            $emailDomain->update([
                'disclaimer_enabled' => $enabled,
                'disclaimer_text' => $text,
            ]);

            Notification::make()
                ->title($enabled ? __('Disclaimer enabled') : __('Disclaimer disabled'))
                ->success()
                ->send();
        } catch (Exception $e) {
            Notification::make()->title(__('Error'))->body(SafeError::message($e))->danger()->send();
        }
    }

    public function spamForm(Schema $schema): Schema
    {
        return $schema
            ->statePath('spamFormData')
            ->schema([
                Section::make(__('Spam Settings'))
                    ->schema([
                        Textarea::make('whitelist')
                            ->label(__('Whitelist (one per line)'))
                            ->rows(6)
                            ->placeholder(__("friend@example.com\ntrusted.com")),
                        Textarea::make('blacklist')
                            ->label(__('Blacklist (one per line)'))
                            ->rows(6)
                            ->placeholder(__("spam@example.com\nbad-domain.com")),
                        TextInput::make('score')
                            ->label(__('Spam Score Threshold'))
                            ->numeric()
                            ->default(6.0)
                            ->helperText(__('Lower values are stricter, higher values are more permissive.')),
                    ])
                    ->columns(2),
            ]);
    }

    protected function loadSpamSettings(): void
    {
        $settings = UserSetting::getForUser(Auth::id(), 'spam_settings', [
            'whitelist' => [],
            'blacklist' => [],
            'score' => 6.0,
        ]);

        $this->spamFormData = [
            'whitelist' => implode("\n", $settings['whitelist'] ?? []),
            'blacklist' => implode("\n", $settings['blacklist'] ?? []),
            'score' => $settings['score'] ?? 6.0,
        ];
    }

    public function saveSpamSettings(): void
    {
        $data = $this->spamForm->getState();
        $whitelist = $this->linesToArray($data['whitelist'] ?? '');
        $blacklist = $this->linesToArray($data['blacklist'] ?? '');
        $score = isset($data['score']) && $data['score'] !== '' ? (float) $data['score'] : null;

        UserSetting::setForUser(Auth::id(), 'spam_settings', [
            'whitelist' => $whitelist,
            'blacklist' => $blacklist,
            'score' => $score,
        ]);

        $result = $this->agent()->rspamdUserSettings($this->getUsername(), $whitelist, $blacklist, $score);
        if (! ($result['success'] ?? false)) {
            Notification::make()
                ->title(__('Failed to update spam settings'))
                ->body($result['error'] ?? '')
                ->danger()
                ->send();

            return;
        }

        Notification::make()
            ->title(__('Spam settings updated'))
            ->success()
            ->send();
    }

    protected function linesToArray(string $value): array
    {
        return collect(preg_split('/\\r\\n|\\r|\\n/', $value))
            ->map(fn ($line) => trim((string) $line))
            ->filter()
            ->values()
            ->toArray();
    }
}
