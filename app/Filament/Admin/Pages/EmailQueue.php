<?php

declare(strict_types=1);

namespace App\Filament\Admin\Pages;

use App\Services\Agent\InteractsWithAgent;
use App\Support\SafeError;
use BackedEnum;
use Filament\Actions\Action;
use Filament\Actions\Concerns\InteractsWithActions;
use Filament\Actions\Contracts\HasActions;
use Filament\Notifications\Notification;
use Filament\Pages\Page;
use Filament\Support\Icons\Heroicon;
use Filament\Tables\Columns\TextColumn;
use Filament\Tables\Concerns\InteractsWithTable;
use Filament\Tables\Contracts\HasTable;
use Filament\Tables\Table;
use Illuminate\Contracts\Support\Htmlable;

class EmailQueue extends Page implements HasActions, HasTable
{
    use InteractsWithActions;
    use InteractsWithAgent;
    use InteractsWithTable;

    protected static string|BackedEnum|null $navigationIcon = Heroicon::OutlinedQueueList;

    protected static ?int $navigationSort = null;

    protected static ?string $slug = 'email-queue';

    protected static bool $shouldRegisterNavigation = false;

    protected string $view = 'filament.admin.pages.email-queue';

    public array $queueItems = [];

    protected bool $queueLoaded = false;

    public function getTitle(): string|Htmlable
    {
        return __('Email Queue Manager');
    }

    public static function getNavigationLabel(): string
    {
        return __('Email Queue');
    }

    public function mount(): void
    {
        $this->redirect(EmailLogs::getUrl());
    }

    public function loadQueue(bool $refreshTable = true): void
    {
        try {
            $result = $this->agent()->call('mail.queue_list');
            $this->queueItems = $result->get('queue', []);
            $this->queueLoaded = true;
        } catch (\Exception $e) {
            $this->queueItems = [];
            $this->queueLoaded = true;
            Notification::make()
                ->title(__('Failed to load mail queue'))
                ->body(SafeError::message($e))
                ->danger()
                ->send();
        }

        if ($refreshTable) {
            $this->resetTable();
        }
    }

    public function table(Table $table): Table
    {
        return $table
            ->records(function () {
                if (! $this->queueLoaded) {
                    $this->loadQueue(false);
                }

                return collect($this->queueItems)
                    ->mapWithKeys(function (array $record, int $index): array {
                        $key = $record['id'] ?? (string) $index;

                        return [$key !== '' ? $key : (string) $index => $record];
                    })
                    ->all();
            })
            ->columns([
                TextColumn::make('id')
                    ->label(__('Queue ID'))
                    ->fontFamily('mono')
                    ->copyable(),
                TextColumn::make('arrival')
                    ->label(__('Arrival')),
                TextColumn::make('sender')
                    ->label(__('Sender'))
                    ->wrap()
                    ->searchable(),
                TextColumn::make('recipients')
                    ->label(__('Recipients'))
                    ->formatStateUsing(function (array $record): string {
                        $recipients = $record['recipients'] ?? [];
                        if (empty($recipients)) {
                            return __('Unknown');
                        }
                        $first = $recipients[0] ?? '';
                        $count = count($recipients);

                        return $count > 1 ? $first.' +'.($count - 1) : $first;
                    })
                    ->wrap(),
                TextColumn::make('size')
                    ->label(__('Size'))
                    ->formatStateUsing(fn (array $record): string => $record['size'] ?? ''),
                TextColumn::make('status')
                    ->label(__('Status'))
                    ->wrap(),
            ])
            ->recordActions([
                Action::make('retry')
                    ->label(__('Retry'))
                    ->icon('heroicon-o-arrow-path')
                    ->color('info')
                    ->action(function (array $record): void {
                        $this->agentCall(
                            action: 'mail.queue_retry',
                            params: ['id' => $record['id'] ?? ''],
                            successTitle: 'Message retried',
                            errorTitle: 'Retry failed',
                            onSuccess: fn () => $this->loadQueue(),
                        );
                    }),
                Action::make('delete')
                    ->label(__('Delete'))
                    ->icon('heroicon-o-trash')
                    ->color('danger')
                    ->requiresConfirmation()
                    ->action(function (array $record): void {
                        $this->agentCall(
                            action: 'mail.queue_delete',
                            params: ['id' => $record['id'] ?? ''],
                            successTitle: 'Message deleted',
                            errorTitle: 'Delete failed',
                            onSuccess: fn () => $this->loadQueue(),
                        );
                    }),
            ])
            ->emptyStateHeading(__('Mail queue is empty'))
            ->emptyStateDescription(__('No deferred messages found.'))
            ->headerActions([
                Action::make('refresh')
                    ->label(__('Refresh'))
                    ->icon('heroicon-o-arrow-path')
                    ->action(fn () => $this->loadQueue()),
            ]);
    }
}
