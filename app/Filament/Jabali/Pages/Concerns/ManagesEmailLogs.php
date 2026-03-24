<?php

declare(strict_types=1);

namespace App\Filament\Jabali\Pages\Concerns;

use Exception;
use Filament\Actions\Action;
use Filament\Infolists\Components\TextEntry;
use Filament\Schemas\Components\Section;
use Filament\Tables\Columns\TextColumn;
use Filament\Tables\Table;

trait ManagesEmailLogs
{
    protected function emailLogsTable(Table $table): Table
    {
        // Read mail logs (last 100 entries)
        $logs = $this->getEmailLogs();

        return $table
            ->records(fn () => $logs)
            ->columns([
                TextColumn::make('timestamp')
                    ->label(__('Time'))
                    ->dateTime('M d, H:i:s')
                    ->sortable(),
                TextColumn::make('status')
                    ->label(__('Status'))
                    ->badge()
                    ->color(fn (array $record) => match ($record['status'] ?? '') {
                        'sent', 'delivered' => 'success',
                        'deferred' => 'warning',
                        'bounced', 'rejected', 'failed' => 'danger',
                        default => 'gray',
                    }),
                TextColumn::make('from')
                    ->label(__('From'))
                    ->limit(30)
                    ->searchable(),
                TextColumn::make('to')
                    ->label(__('To'))
                    ->limit(30)
                    ->searchable(),
                TextColumn::make('subject')
                    ->label(__('Subject'))
                    ->limit(40)
                    ->placeholder(__('(no subject)')),
            ])
            ->recordActions([
                Action::make('details')
                    ->label(__('Details'))
                    ->icon('heroicon-o-information-circle')
                    ->color('gray')
                    ->modalHeading(__('Email Details'))
                    ->modalSubmitAction(false)
                    ->modalCancelActionLabel(__('Close'))
                    ->infolist(fn (array $record): array => [
                        Section::make(__('Message Info'))
                            ->columns(2)
                            ->schema([
                                TextEntry::make('from')
                                    ->label(__('From'))
                                    ->state($record['from'] ?? '-')
                                    ->copyable(),
                                TextEntry::make('to')
                                    ->label(__('To'))
                                    ->state($record['to'] ?? '-')
                                    ->copyable(),
                                TextEntry::make('subject')
                                    ->label(__('Subject'))
                                    ->state($record['subject'] ?? '-'),
                                TextEntry::make('timestamp')
                                    ->label(__('Time'))
                                    ->state(isset($record['timestamp']) ? date('Y-m-d H:i:s', $record['timestamp']) : '-'),
                            ]),
                        Section::make(__('Delivery Status'))
                            ->schema([
                                TextEntry::make('status')
                                    ->label(__('Status'))
                                    ->state($record['status'] ?? '-')
                                    ->badge()
                                    ->color(match ($record['status'] ?? '') {
                                        'sent', 'delivered' => 'success',
                                        'deferred' => 'warning',
                                        'bounced', 'rejected', 'failed' => 'danger',
                                        default => 'gray',
                                    }),
                                TextEntry::make('message')
                                    ->label(__('Message'))
                                    ->state($record['message'] ?? '-'),
                            ]),
                    ]),
            ])
            ->emptyStateHeading(__('No email logs'))
            ->emptyStateDescription(__('Email activity will appear here once emails are sent or received.'))
            ->emptyStateIcon('heroicon-o-document-text')
            ->striped()
            ->defaultSort('timestamp', 'desc');
    }

    public function getEmailLogs(): array
    {
        try {
            $result = $this->agent()->call('email.get_logs', [
                'username' => $this->getUsername(),
                'limit' => 100,
            ]);

            return $result->get('logs', []);
        } catch (Exception $e) {
            return [];
        }
    }
}
