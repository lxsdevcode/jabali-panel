<?php

declare(strict_types=1);

namespace App\Filament\Jabali\Pages\Concerns;

use App\Models\Autoresponder;
use App\Models\Mailbox;
use App\Support\SafeError;
use Exception;
use Filament\Actions\Action;
use Filament\Forms\Components\DatePicker;
use Filament\Forms\Components\Select;
use Filament\Forms\Components\Textarea;
use Filament\Forms\Components\TextInput;
use Filament\Forms\Components\Toggle;
use Filament\Notifications\Notification;
use Filament\Tables\Columns\TextColumn;
use Filament\Tables\Table;
use Illuminate\Database\Eloquent\Builder;
use Illuminate\Support\Facades\Auth;

trait ManagesAutoresponders
{
    protected function autorespondersTable(Table $table): Table
    {
        return $table
            ->query(
                Autoresponder::query()
                    ->whereHas('mailbox.emailDomain.domain', fn (Builder $q) => $q->where('user_id', Auth::id()))
                    ->with('mailbox.emailDomain.domain')
            )
            ->columns([
                TextColumn::make('mailbox.email')
                    ->label(__('Email'))
                    ->icon('heroicon-o-envelope')
                    ->iconColor('primary')
                    ->searchable()
                    ->sortable(),
                TextColumn::make('subject')
                    ->label(__('Subject'))
                    ->limit(30)
                    ->searchable(),
                TextColumn::make('status')
                    ->label(__('Status'))
                    ->badge()
                    ->getStateUsing(function (Autoresponder $record): string {
                        if (! $record->is_active) {
                            return __('Disabled');
                        }
                        if ($record->isCurrentlyActive()) {
                            return __('Active');
                        }
                        if ($record->start_date && now()->lt($record->start_date)) {
                            return __('Scheduled');
                        }

                        return __('Expired');
                    })
                    ->color(function (Autoresponder $record): string {
                        if (! $record->is_active) {
                            return 'gray';
                        }
                        if ($record->isCurrentlyActive()) {
                            return 'success';
                        }
                        if ($record->start_date && now()->lt($record->start_date)) {
                            return 'warning';
                        }

                        return 'danger';
                    }),
                TextColumn::make('start_date')
                    ->label(__('From'))
                    ->date('M d, Y')
                    ->placeholder(__('No start date')),
                TextColumn::make('end_date')
                    ->label(__('Until'))
                    ->date('M d, Y')
                    ->placeholder(__('No end date')),
            ])
            ->recordActions([
                Action::make('edit')
                    ->label(__('Edit'))
                    ->icon('heroicon-o-pencil')
                    ->color('info')
                    ->modalHeading(__('Edit Autoresponder'))
                    ->modalDescription(fn (Autoresponder $record) => $record->mailbox->email)
                    ->modalIcon('heroicon-o-clock')
                    ->modalIconColor('info')
                    ->modalSubmitActionLabel(__('Save Changes'))
                    ->fillForm(fn (Autoresponder $record) => [
                        'subject' => $record->subject,
                        'message' => $record->message,
                        'start_date' => $record->start_date?->format('Y-m-d'),
                        'end_date' => $record->end_date?->format('Y-m-d'),
                        'is_active' => $record->is_active,
                    ])
                    ->form([
                        TextInput::make('subject')
                            ->label(__('Subject'))
                            ->required()
                            ->maxLength(255),
                        Textarea::make('message')
                            ->label(__('Message'))
                            ->required()
                            ->rows(5)
                            ->helperText(__('The automatic reply message')),
                        DatePicker::make('start_date')
                            ->label(__('Start Date'))
                            ->helperText(__('Leave empty to start immediately')),
                        DatePicker::make('end_date')
                            ->label(__('End Date'))
                            ->helperText(__('Leave empty for no end date')),
                        Toggle::make('is_active')
                            ->label(__('Active'))
                            ->default(true),
                    ])
                    ->action(fn (Autoresponder $record, array $data) => $this->updateAutoresponder($record, $data)),
                Action::make('toggle')
                    ->label(fn (Autoresponder $record) => $record->is_active ? __('Disable') : __('Enable'))
                    ->icon(fn (Autoresponder $record) => $record->is_active ? 'heroicon-o-pause' : 'heroicon-o-play')
                    ->color('gray')
                    ->action(fn (Autoresponder $record) => $this->toggleAutoresponder($record)),
                Action::make('delete')
                    ->label(__('Delete'))
                    ->icon('heroicon-o-trash')
                    ->color('danger')
                    ->requiresConfirmation()
                    ->modalHeading(__('Delete Autoresponder'))
                    ->modalDescription(fn (Autoresponder $record) => __("Delete autoresponder for ':email'?", ['email' => $record->mailbox->email]))
                    ->modalIcon('heroicon-o-trash')
                    ->modalIconColor('danger')
                    ->modalSubmitActionLabel(__('Delete'))
                    ->action(fn (Autoresponder $record) => $this->deleteAutoresponder($record)),
            ])
            ->emptyStateHeading(__('No autoresponders'))
            ->emptyStateDescription(__('Set up vacation messages for your mailboxes.'))
            ->emptyStateIcon('heroicon-o-clock')
            ->striped();
    }

    protected function createAutoresponderAction(): Action
    {
        return Action::make('createAutoresponder')
            ->label(__('New Autoresponder'))
            ->icon('heroicon-o-clock')
            ->color('warning')
            ->visible(fn () => Mailbox::whereHas('emailDomain.domain', fn ($q) => $q->where('user_id', Auth::id()))->exists())
            ->modalHeading(__('Create Autoresponder'))
            ->modalDescription(__('Set up an automatic vacation reply'))
            ->modalIcon('heroicon-o-clock')
            ->modalIconColor('warning')
            ->modalSubmitActionLabel(__('Create'))
            ->form([
                Select::make('mailbox_id')
                    ->label(__('Mailbox'))
                    ->options(fn () => Mailbox::whereHas('emailDomain.domain', fn ($q) => $q->where('user_id', Auth::id()))
                        ->with('emailDomain.domain')
                        ->get()
                        ->mapWithKeys(fn ($m) => [$m->id => $m->email])
                        ->toArray())
                    ->required()
                    ->searchable(),
                TextInput::make('subject')
                    ->label(__('Subject'))
                    ->required()
                    ->default(__('Out of Office'))
                    ->maxLength(255),
                Textarea::make('message')
                    ->label(__('Message'))
                    ->required()
                    ->rows(5)
                    ->default(__("Thank you for your email. I am currently out of the office and will respond to your message upon my return.\n\nBest regards"))
                    ->helperText(__('The automatic reply message')),
                DatePicker::make('start_date')
                    ->label(__('Start Date'))
                    ->helperText(__('Leave empty to start immediately')),
                DatePicker::make('end_date')
                    ->label(__('End Date'))
                    ->helperText(__('Leave empty for no end date')),
            ])
            ->action(function (array $data): void {
                $mailbox = Mailbox::whereHas('emailDomain.domain', fn ($q) => $q->where('user_id', Auth::id()))
                    ->find($data['mailbox_id']);

                if (! $mailbox) {
                    Notification::make()->title(__('Mailbox not found'))->danger()->send();

                    return;
                }

                // Check if autoresponder already exists for this mailbox
                if (Autoresponder::where('mailbox_id', $mailbox->id)->exists()) {
                    Notification::make()
                        ->title(__('Autoresponder already exists'))
                        ->body(__('Edit the existing autoresponder instead.'))
                        ->danger()
                        ->send();

                    return;
                }

                try {
                    // Create autoresponder in database
                    $autoresponder = Autoresponder::create([
                        'mailbox_id' => $mailbox->id,
                        'subject' => $data['subject'],
                        'message' => $data['message'],
                        'start_date' => $data['start_date'] ?? null,
                        'end_date' => $data['end_date'] ?? null,
                        'is_active' => true,
                    ]);

                    // Configure on mail server via agent
                    $this->agent()->send('email.autoresponder_set', [
                        'username' => $this->getUsername(),
                        'email' => $mailbox->email,
                        'subject' => $data['subject'],
                        'message' => $data['message'],
                        'start_date' => $data['start_date'] ?? null,
                        'end_date' => $data['end_date'] ?? null,
                        'active' => true,
                    ]);

                    Notification::make()->title(__('Autoresponder created'))->success()->send();

                    $this->setTab('autoresponders');
                } catch (Exception $e) {
                    Notification::make()->title(__('Error'))->body(SafeError::message($e))->danger()->send();
                }
            });
    }

    public function updateAutoresponder(Autoresponder $autoresponder, array $data): void
    {
        try {
            $autoresponder->update([
                'subject' => $data['subject'],
                'message' => $data['message'],
                'start_date' => $data['start_date'] ?? null,
                'end_date' => $data['end_date'] ?? null,
                'is_active' => $data['is_active'] ?? true,
            ]);

            // Update on mail server
            $this->agent()->send('email.autoresponder_set', [
                'username' => $this->getUsername(),
                'email' => $autoresponder->mailbox->email,
                'subject' => $data['subject'],
                'message' => $data['message'],
                'start_date' => $data['start_date'] ?? null,
                'end_date' => $data['end_date'] ?? null,
                'active' => $data['is_active'] ?? true,
            ]);

            Notification::make()->title(__('Autoresponder updated'))->success()->send();
        } catch (Exception $e) {
            Notification::make()->title(__('Error'))->body(SafeError::message($e))->danger()->send();
        }
    }

    public function toggleAutoresponder(Autoresponder $autoresponder): void
    {
        try {
            $newStatus = ! $autoresponder->is_active;
            $autoresponder->update(['is_active' => $newStatus]);

            // Update on mail server
            $this->agent()->send('email.autoresponder_toggle', [
                'username' => $this->getUsername(),
                'email' => $autoresponder->mailbox->email,
                'active' => $newStatus,
            ]);

            Notification::make()
                ->title($newStatus ? __('Autoresponder enabled') : __('Autoresponder disabled'))
                ->success()
                ->send();
        } catch (Exception $e) {
            Notification::make()->title(__('Error'))->body(SafeError::message($e))->danger()->send();
        }
    }

    public function deleteAutoresponder(Autoresponder $autoresponder): void
    {
        try {
            // Remove from mail server
            $this->agent()->send('email.autoresponder_delete', [
                'username' => $this->getUsername(),
                'email' => $autoresponder->mailbox->email,
            ]);

            $autoresponder->delete();

            Notification::make()->title(__('Autoresponder deleted'))->success()->send();
        } catch (Exception $e) {
            Notification::make()->title(__('Error'))->body(SafeError::message($e))->danger()->send();
        }
    }
}
