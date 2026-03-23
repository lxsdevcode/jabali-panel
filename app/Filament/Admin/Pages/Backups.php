<?php

declare(strict_types=1);

namespace App\Filament\Admin\Pages;

use App\Models\Backup;
use App\Models\BackupDestination;
use App\Models\BackupSchedule;
use App\Models\DnsSetting;
use App\Models\User;
use App\Services\Agent\InteractsWithAgent;
use App\Support\SafeError;
use App\Support\ServerFacts;
use BackedEnum;
use Exception;
use Filament\Actions\Action;
use Filament\Actions\Concerns\InteractsWithActions;
use Filament\Actions\Contracts\HasActions;
use Filament\Forms\Components\Radio;
use Filament\Forms\Components\Select;
use Filament\Forms\Components\Textarea;
use Filament\Forms\Components\TextInput;
use Filament\Forms\Components\Toggle;
use Filament\Forms\Concerns\InteractsWithForms;
use Filament\Forms\Contracts\HasForms;
use Filament\Notifications\Notification;
use Filament\Pages\Page;
use Filament\Schemas\Components\Actions as FormActions;
use Filament\Schemas\Components\Grid;
use Filament\Schemas\Components\Section;
use Filament\Schemas\Components\Tabs;
use Filament\Schemas\Components\Tabs\Tab;
use Filament\Schemas\Components\View;
use Filament\Schemas\Schema;
use Filament\Tables\Columns\TextColumn;
use Filament\Tables\Columns\ViewColumn;
use Filament\Tables\Concerns\InteractsWithTable;
use Filament\Tables\Contracts\HasTable;
use Filament\Tables\Table;
use Illuminate\Contracts\Support\Htmlable;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Support\Facades\File;
use Illuminate\Support\Facades\Log;
use Livewire\Attributes\Url;

class Backups extends Page implements HasActions, HasForms, HasTable
{
    use InteractsWithActions;
    use InteractsWithAgent;
    use InteractsWithForms;
    use InteractsWithTable;

    protected static string|BackedEnum|null $navigationIcon = 'heroicon-o-cloud-arrow-up';

    protected static ?int $navigationSort = 11;

    public static function getNavigationLabel(): string
    {
        return __('Backups');
    }

    protected string $view = 'filament.admin.pages.backups';

    #[Url(as: 'tab')]
    public ?string $activeTab = 'destinations';

    public function getTitle(): string|Htmlable
    {
        return __('Server Backups');
    }

    public function mount(): void
    {
        $this->activeTab = $this->normalizeTabName($this->activeTab);
    }

    protected function normalizeTabName(?string $tab): string
    {
        return match ($tab) {
            'destinations', 'schedules', 'backups', 'logs' => $tab,
            default => 'destinations',
        };
    }

    public function setTab(string $tab): void
    {
        $this->activeTab = $this->normalizeTabName($tab);
        $this->resetTable();
    }

    public function updatedActiveTab(): void
    {
        $this->activeTab = $this->normalizeTabName($this->activeTab);
        $this->resetTable();
    }

    protected function getForms(): array
    {
        return [
            'backupsForm',
        ];
    }

    public function backupsForm(Schema $schema): Schema
    {
        return $schema
            ->schema([
                Section::make(__('Recommendation'))
                    ->description(__('Use Incremental Backups for scheduled server backups. They only store changes since the last backup, significantly reducing storage space and backup time while maintaining full restore capability.'))
                    ->icon('heroicon-o-light-bulb')
                    ->iconColor('info')
                    ->collapsed(false)
                    ->collapsible(false),
                Tabs::make(__('Backup Sections'))
                    ->contained()
                    ->livewireProperty('activeTab')
                    ->tabs([
                        'destinations' => Tab::make(__('Destinations'))
                            ->icon('heroicon-o-server-stack')
                            ->schema([
                                View::make('filament.admin.pages.backups-tab-table'),
                            ]),
                        'schedules' => Tab::make(__('Schedules'))
                            ->icon('heroicon-o-calendar-days')
                            ->schema([
                                View::make('filament.admin.pages.backups-tab-table'),
                            ]),
                        'backups' => Tab::make(__('Snapshots / Restore'))
                            ->icon('heroicon-o-archive-box')
                            ->schema([
                                View::make('filament.admin.pages.backups-tab-table'),
                            ]),
                        'logs' => Tab::make(__('Logs'))
                            ->icon('heroicon-o-document-text')
                            ->schema([
                                View::make('filament.admin.pages.backups-tab-table'),
                            ]),
                    ]),
            ]);
    }

    protected function logsTable(Table $table): Table
    {
        return $table
            ->records(function (int $page, int $recordsPerPage): \Illuminate\Pagination\LengthAwarePaginator {
                $allEntries = $this->readBackupLogEntries();
                $collection = collect($allEntries)->values();

                return new \Illuminate\Pagination\LengthAwarePaginator(
                    $collection->forPage($page, $recordsPerPage)->values(),
                    total: $collection->count(),
                    perPage: $recordsPerPage,
                    currentPage: $page,
                    options: ['pageName' => 'logsPage'],
                );
            })
            ->columns([
                TextColumn::make('timestamp')
                    ->label(__('Time'))
                    ->fontFamily('mono')
                    ->color('gray')
                    ->sortable(),
                TextColumn::make('level')
                    ->label(__('Level'))
                    ->badge()
                    ->color(fn (string $state, array $record): string => match ($state) {
                        'ERROR', 'CRITICAL', 'ALERT', 'EMERGENCY' => 'danger',
                        'WARNING' => 'warning',
                        'INFO' => str_contains($record['message'] ?? '', 'successfully') ? 'success' : 'gray',
                        'DEBUG' => 'info',
                        default => 'gray',
                    })
                    ->formatStateUsing(fn (string $state, array $record): string => match ($state) {
                        'INFO' => str_contains($record['message'] ?? '', 'successfully') ? 'SUCCESS' : 'INFO',
                        default => $state,
                    }),
                TextColumn::make('message')
                    ->label(__('Message'))
                    ->wrap()
                    ->lineClamp(3)
                    ->tooltip(fn (array $record): string => $record['message']),
            ])
            ->filters([
                \Filament\Tables\Filters\SelectFilter::make('level')
                    ->label(__('Level'))
                    ->options([
                        'ERROR' => __('Errors'),
                        'WARNING' => __('Warnings'),
                        'INFO' => __('Info'),
                        'DEBUG' => __('Debug'),
                    ])
                    ->multiple()
                    ->query(fn () => null),
            ])
            ->headerActions([
                Action::make('clearLogs')
                    ->label(__('Clear Logs'))
                    ->icon('heroicon-o-trash')
                    ->color('danger')
                    ->size('sm')
                    ->requiresConfirmation()
                    ->modalDescription(__('This will remove all backup log entries from the log file. This action cannot be undone.'))
                    ->action(function () {
                        $this->clearBackupLogs();
                        Notification::make()->title(__('Backup logs cleared'))->success()->send();
                    }),
            ])
            ->defaultSort('timestamp', 'desc')
            ->emptyStateHeading(__('No backup logs found'))
            ->emptyStateDescription(__('Backup activity will appear here once backups start running.'))
            ->emptyStateIcon('heroicon-o-document-text')
            ->striped()
            ->defaultPaginationPageOption(25)
            ->queryStringIdentifier('logs')
            ->poll('30s');
    }

    /**
     * Read backup-related entries from the Laravel log file.
     *
     * @return array<int, array{id: int, level: string, timestamp: string, message: string}>
     */
    protected function readBackupLogEntries(): array
    {
        $logFile = storage_path('logs/laravel.log');

        if (! file_exists($logFile)) {
            return [];
        }

        $prefixes = ['RunServerBackup:', 'RunRestore:', 'IndexRemoteBackups:', 'RunBackupSchedules:', 'BackupSchedule:', 'BackupDestination:'];
        $backupKeywords = ['Backup', 'Restore', 'retention'];
        $retentionDays = (int) DnsSetting::get('backup_log_retention_days', 60);
        $cutoffDate = now()->subDays($retentionDays)->format('Y-m-d H:i:s');

        // Get active level filter from table state
        $levelFilter = $this->tableFilters['level']['values'] ?? [];

        // Read last 500KB of the log file
        $maxBytes = 500 * 1024;
        $fileSize = filesize($logFile);
        $offset = max(0, $fileSize - $maxBytes);

        $handle = fopen($logFile, 'r');
        if (! $handle) {
            return [];
        }

        if ($offset > 0) {
            fseek($handle, $offset);
            fgets($handle);
        }

        $entries = [];
        $currentEntry = null;
        $id = 0;

        while (($line = fgets($handle)) !== false) {
            $line = rtrim($line);

            if (preg_match('/^\[(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})\] \S+\.(INFO|WARNING|ERROR|DEBUG|CRITICAL|ALERT|EMERGENCY):\s*(.*)$/', $line, $matches)) {
                if ($currentEntry !== null) {
                    $entries[$currentEntry['id']] = $currentEntry;
                }

                $currentEntry = null;
                $timestamp = $matches[1];
                $level = $matches[2];
                $message = $matches[3];

                // Skip entries older than retention
                if ($timestamp < $cutoffDate) {
                    continue;
                }

                // Apply level filter
                if (! empty($levelFilter)) {
                    $normalizedLevel = in_array($level, ['CRITICAL', 'ALERT', 'EMERGENCY']) ? 'ERROR' : $level;
                    if (! in_array($normalizedLevel, $levelFilter)) {
                        continue;
                    }
                }

                $isBackup = false;
                foreach ($prefixes as $prefix) {
                    if (str_contains($message, $prefix)) {
                        $isBackup = true;
                        break;
                    }
                }

                if (! $isBackup) {
                    foreach ($backupKeywords as $keyword) {
                        if (str_contains($message, $keyword)) {
                            $isBackup = true;
                            break;
                        }
                    }
                }

                if ($isBackup) {
                    $id++;
                    $currentEntry = [
                        'id' => $id,
                        'timestamp' => $timestamp,
                        'level' => $level,
                        'message' => $message,
                    ];
                }
            } elseif ($currentEntry !== null && $line !== '') {
                $currentEntry['message'] .= "\n".$line;
            }
        }

        fclose($handle);

        if ($currentEntry !== null) {
            $entries[$currentEntry['id']] = $currentEntry;
        }

        return array_reverse($entries, true);
    }

    /**
     * Remove backup-related log entries from the Laravel log file.
     */
    protected function clearBackupLogs(): void
    {
        $logFile = storage_path('logs/laravel.log');

        if (! file_exists($logFile)) {
            return;
        }

        $prefixes = ['RunServerBackup:', 'RunRestore:', 'IndexRemoteBackups:', 'RunBackupSchedules:', 'BackupSchedule:', 'BackupDestination:'];
        $backupKeywords = ['Backup', 'Restore', 'retention'];

        $content = file_get_contents($logFile);
        if ($content === false) {
            return;
        }

        $lines = explode("\n", $content);
        $filtered = [];
        $skipMultiline = false;

        foreach ($lines as $line) {
            if (preg_match('/^\[\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\] \S+\.(INFO|WARNING|ERROR|DEBUG|CRITICAL|ALERT|EMERGENCY):\s*(.*)$/', $line, $matches)) {
                $message = $matches[2];
                $isBackup = false;

                foreach ($prefixes as $prefix) {
                    if (str_contains($message, $prefix)) {
                        $isBackup = true;
                        break;
                    }
                }

                if (! $isBackup) {
                    foreach ($backupKeywords as $keyword) {
                        if (str_contains($message, $keyword)) {
                            $isBackup = true;
                            break;
                        }
                    }
                }

                $skipMultiline = $isBackup;
                if (! $isBackup) {
                    $filtered[] = $line;
                }
            } elseif (! $skipMultiline) {
                $filtered[] = $line;
            }
        }

        file_put_contents($logFile, implode("\n", $filtered), LOCK_EX);
        Log::info('BackupDestination: Backup logs cleared by admin');
    }

    protected function supportsIncremental($destinationId): bool
    {
        if (empty($destinationId)) {
            return false;
        }

        $destination = BackupDestination::find($destinationId);
        if (! $destination) {
            return false;
        }

        return in_array($destination->type, ['sftp', 'nfs']);
    }

    public function table(Table $table): Table
    {
        return match ($this->activeTab) {
            'destinations' => $this->destinationsTable($table),
            'schedules' => $this->schedulesTable($table),
            'backups' => $this->backupsTable($table),
            'logs' => $this->logsTable($table),
            default => $this->destinationsTable($table),
        };
    }

    protected function destinationsTable(Table $table): Table
    {
        return $table
            ->query(BackupDestination::query()->where('is_server_backup', true)->orderBy('name'))
            ->columns([
                TextColumn::make('name')
                    ->label(__('Name'))
                    ->weight('medium')
                    ->description(fn (BackupDestination $record): ?string => $record->is_default ? __('Default') : null)
                    ->searchable(),
                TextColumn::make('type')
                    ->label(__('Type'))
                    ->badge()
                    ->formatStateUsing(fn (string $state): string => strtoupper($state))
                    ->color(fn (string $state): string => match ($state) {
                        'sftp' => 'info',
                        'nfs' => 'warning',
                        's3' => 'success',
                        default => 'gray',
                    }),
                TextColumn::make('test_status')
                    ->label(__('Status'))
                    ->badge()
                    ->formatStateUsing(fn (?string $state): string => match ($state) {
                        'success' => __('Connected'),
                        'failed' => __('Failed'),
                        default => __('Not Tested'),
                    })
                    ->color(fn (?string $state): string => match ($state) {
                        'success' => 'success',
                        'failed' => 'danger',
                        default => 'gray',
                    }),
                TextColumn::make('last_tested_at')
                    ->label(__('Last Tested'))
                    ->since()
                    ->placeholder(__('Never'))
                    ->color('gray'),
            ])
            ->recordActions([
                Action::make('edit')
                    ->label(__('Edit'))
                    ->icon('heroicon-o-pencil-square')
                    ->color('gray')
                    ->size('sm')
                    ->fillForm(function (BackupDestination $record): array {
                        $config = $record->config ?? [];

                        return array_merge($config, [
                            'name' => $record->name,
                            'type' => $record->type,
                            'is_default' => $record->is_default,
                        ]);
                    })
                    ->form($this->getDestinationForm())
                    ->action(fn (array $data, BackupDestination $record) => $this->updateDestination($record, $data)),
                Action::make('test')
                    ->label(__('Test'))
                    ->icon('heroicon-o-check-circle')
                    ->color('success')
                    ->size('sm')
                    ->action(fn (BackupDestination $record) => $this->testDestination($record->id)),
                Action::make('delete')
                    ->label(__('Delete'))
                    ->icon('heroicon-o-trash')
                    ->color('danger')
                    ->size('sm')
                    ->requiresConfirmation()
                    ->action(fn (BackupDestination $record) => $this->deleteDestination($record->id)),
            ])
            ->emptyStateHeading(__('No remote destinations configured'))
            ->emptyStateDescription(__('Click "Add Destination" to configure SFTP, NFS, or S3 storage'))
            ->emptyStateIcon('heroicon-o-server-stack')
            ->striped();
    }

    protected function schedulesTable(Table $table): Table
    {
        return $table
            ->query(BackupSchedule::query()->where('is_server_backup', true)->with('destination')->orderBy('name'))
            ->columns([
                TextColumn::make('name')
                    ->label(__('Name'))
                    ->weight('medium')
                    ->searchable(),
                TextColumn::make('frequency_label')
                    ->label(__('Frequency')),
                TextColumn::make('destination.name')
                    ->label(__('Destination'))
                    ->placeholder(__('Local')),
                TextColumn::make('retention_count')
                    ->label(__('Retention'))
                    ->formatStateUsing(fn (int $state): string => $state.' '.__('backups')),
                TextColumn::make('last_run_at')
                    ->label(__('Last Run'))
                    ->since()
                    ->dateTimeTooltip('M j, Y H:i T', timezone: $this->getSystemTimezone())
                    ->placeholder(__('Never'))
                    ->color('gray'),
                TextColumn::make('next_run_at')
                    ->label(__('Next Run'))
                    ->since()
                    ->dateTimeTooltip('M j, Y H:i T', timezone: $this->getSystemTimezone())
                    ->placeholder(__('Not scheduled'))
                    ->color('gray'),
                ViewColumn::make('status')
                    ->label(__('Status'))
                    ->view('filament.admin.columns.schedule-status'),
            ])
            ->recordActions([
                Action::make('run')
                    ->label(__('Run'))
                    ->icon('heroicon-o-play')
                    ->color('gray')
                    ->size('sm')
                    ->visible(fn (BackupSchedule $record): bool => ! Backup::where('schedule_id', $record->id)->running()->exists())
                    ->action(fn (BackupSchedule $record) => $this->runScheduleNow($record->id)),
                Action::make('stop')
                    ->label(__('Stop'))
                    ->icon('heroicon-o-stop')
                    ->color('danger')
                    ->size('sm')
                    ->visible(fn (BackupSchedule $record): bool => Backup::where('schedule_id', $record->id)->running()->exists())
                    ->requiresConfirmation()
                    ->action(fn (BackupSchedule $record) => $this->stopScheduleBackup($record->id)),
                Action::make('edit')
                    ->label(__('Edit'))
                    ->icon('heroicon-o-pencil')
                    ->color('gray')
                    ->size('sm')
                    ->fillForm(fn (BackupSchedule $record): array => [
                        'name' => $record->name,
                        'backup_type' => $record->metadata['backup_type'] ?? 'full',
                        'frequency' => $record->frequency,
                        'time' => $record->time,
                        'day_of_week' => $record->day_of_week,
                        'day_of_month' => $record->day_of_month,
                        'destination_id' => $record->destination_id ?? '',
                        'retention_count' => $record->retention_count,
                        'include_files' => $record->include_files,
                        'include_databases' => $record->include_databases,
                        'include_mailboxes' => $record->include_mailboxes,
                        'include_dns' => $record->include_dns,
                        'include_ssl' => $record->include_ssl ?? true,
                    ])
                    ->form([
                        TextInput::make('name')->label(__('Schedule Name'))->required(),
                        Select::make('destination_id')
                            ->label(__('Destination'))
                            ->options(fn () => BackupDestination::where('is_server_backup', true)
                                ->where('is_active', true)
                                ->pluck('name', 'id')
                                ->prepend(__('Local Storage'), ''))
                            ->default('')
                            ->live(),
                        Radio::make('backup_type')
                            ->label(__('Backup Type'))
                            ->options(fn ($get) => $this->supportsIncremental($get('destination_id'))
                                ? ['incremental' => __('Incremental'), 'full' => __('Full')]
                                : ['full' => __('Full')])
                            ->required(),
                        Select::make('frequency')
                            ->label(__('Frequency'))
                            ->options(['hourly' => __('Hourly'), 'daily' => __('Daily'), 'weekly' => __('Weekly'), 'monthly' => __('Monthly')])
                            ->required()
                            ->live(),
                        TextInput::make('time')->label(__('Time (HH:MM)'))->visible(fn ($get) => in_array($get('frequency'), ['daily', 'weekly', 'monthly'])),
                        Select::make('day_of_week')
                            ->label(__('Day of Week'))
                            ->options([0 => __('Sunday'), 1 => __('Monday'), 2 => __('Tuesday'), 3 => __('Wednesday'), 4 => __('Thursday'), 5 => __('Friday'), 6 => __('Saturday')])
                            ->visible(fn ($get) => $get('frequency') === 'weekly'),
                        Select::make('day_of_month')
                            ->label(__('Day of Month'))
                            ->options(array_combine(range(1, 28), range(1, 28)))
                            ->visible(fn ($get) => $get('frequency') === 'monthly'),
                        TextInput::make('retention_count')->label(__('Keep Last N Backups'))->numeric(),
                        Section::make(__('Include'))
                            ->schema([
                                Grid::make(2)->schema([
                                    Toggle::make('include_files')->label(__('Website Files')),
                                    Toggle::make('include_databases')->label(__('Databases')),
                                    Toggle::make('include_mailboxes')->label(__('Mailboxes')),
                                    Toggle::make('include_dns')->label(__('DNS Records')),
                                    Toggle::make('include_ssl')->label(__('SSL Certificates')),
                                ]),
                            ]),
                    ])
                    ->action(function (array $data, BackupSchedule $record) {
                        $record->update([
                            'name' => $data['name'],
                            'frequency' => $data['frequency'],
                            'time' => $data['time'] ?? '02:00',
                            'day_of_week' => $data['day_of_week'] ?? null,
                            'day_of_month' => $data['day_of_month'] ?? null,
                            'destination_id' => ! empty($data['destination_id']) ? $data['destination_id'] : null,
                            'retention_count' => $data['retention_count'] ?? 7,
                            'include_files' => $data['include_files'] ?? true,
                            'include_databases' => $data['include_databases'] ?? true,
                            'include_mailboxes' => $data['include_mailboxes'] ?? true,
                            'include_dns' => $data['include_dns'] ?? true,
                            'include_ssl' => $data['include_ssl'] ?? true,
                            'metadata' => array_merge($record->metadata ?? [], ['backup_type' => $data['backup_type'] ?? 'full']),
                        ]);

                        $record->calculateNextRun();
                        $record->save();

                        Notification::make()->title(__('Schedule updated'))->success()->send();
                        $this->resetTable();
                    }),
                Action::make('toggle')
                    ->label(fn (BackupSchedule $record): string => $record->is_active ? __('Disable') : __('Enable'))
                    ->icon(fn (BackupSchedule $record): string => $record->is_active ? 'heroicon-o-pause' : 'heroicon-o-play')
                    ->color('gray')
                    ->size('sm')
                    ->action(fn (BackupSchedule $record) => $this->toggleSchedule($record->id)),
                Action::make('delete')
                    ->label(__('Delete'))
                    ->icon('heroicon-o-trash')
                    ->color('danger')
                    ->size('sm')
                    ->requiresConfirmation()
                    ->action(fn (BackupSchedule $record) => $this->deleteSchedule($record->id)),
            ])
            ->headerActions([
                $this->addScheduleAction(),
            ])
            ->emptyStateHeading(__('No backup schedules configured'))
            ->emptyStateDescription(__('Click "Add Schedule" to set up automatic backups'))
            ->emptyStateIcon('heroicon-o-clock')
            ->striped()
            ->poll(fn () => Backup::running()->exists() ? '3s' : null);
    }

    protected function backupsTable(Table $table): Table
    {
        return $table
            ->query(Backup::query()->where('type', 'server')->with(['destination', 'user'])->orderByDesc('created_at')->limit(50))
            ->columns([
                TextColumn::make('name')
                    ->label(__('Name'))
                    ->weight('medium')
                    ->searchable()
                    ->limit(40),
                ViewColumn::make('status')
                    ->label(__('Status'))
                    ->view('filament.admin.columns.backup-status'),
                TextColumn::make('size_bytes')
                    ->label(__('Size'))
                    ->formatStateUsing(fn (Backup $record): string => $record->size_human),
                TextColumn::make('destination.name')
                    ->label(__('Destination'))
                    ->placeholder(__('Local')),
                TextColumn::make('created_at')
                    ->label(__('Created'))
                    ->dateTime('M j, Y H:i')
                    ->color('gray'),
                TextColumn::make('duration')
                    ->label(__('Duration'))
                    ->placeholder(__('-'))
                    ->color('gray'),
            ])
            ->recordActions([
                Action::make('restore')
                    ->label(__('Restore'))
                    ->icon('heroicon-o-arrow-path')
                    ->color('warning')
                    ->size('sm')
                    ->visible(fn (Backup $record): bool => $record->status === 'completed' && ($record->local_path || $record->remote_path))
                    ->modalHeading(__('Restore Backup'))
                    ->modalDescription(__('Select what you want to restore. Warning: Existing data may be overwritten.'))
                    ->modalWidth('xl')
                    ->form(function (Backup $record): array {
                        $isRemoteBackup = ! $record->local_path || ! file_exists($record->local_path);
                        $manifest = $this->getBackupManifest($record);
                        $users = $manifest['users'] ?? $record->users ?? [];
                        if (empty($users)) {
                            $users = [$manifest['username'] ?? ''];
                        }
                        $users = array_filter($users);
                        $isServerBackup = ($manifest['type'] ?? $record->type) === 'server' && count($users) > 1;
                        $selectedUser = $users[0] ?? '';

                        // Helper to get contents for a specific user (cached per request)
                        $getContentsForUser = function (?string $username) use ($record, $isRemoteBackup, $manifest): array {
                            if (empty($username) || $username === '__all__') {
                                return $manifest;
                            }

                            static $cache = [];
                            $cacheKey = $record->id.'_'.$username;
                            if (isset($cache[$cacheKey])) {
                                return $cache[$cacheKey];
                            }

                            // For local backups, use manifest
                            if (! $isRemoteBackup) {
                                $result = $this->getBackupManifest($record, $username);
                                $cache[$cacheKey] = $result;

                                return $result;
                            }

                            // For remote backups, call agent to list contents via SSH
                            if ($record->destination) {
                                try {
                                    $destConfig = array_merge(
                                        $record->destination->config ?? [],
                                        ['type' => $record->destination->type]
                                    );
                                    $contents = $this->agent()->backupListContents(
                                        $record->remote_path ?? '',
                                        $username,
                                        $destConfig
                                    );
                                    if ($contents['success'] ?? false) {
                                        $cache[$cacheKey] = $contents;

                                        return $contents;
                                    }
                                } catch (Exception $e) {
                                    // Fall back
                                }
                            }

                            $cache[$cacheKey] = $manifest;

                            return $manifest;
                        };

                        $schema = [];

                        // Step 1: User selection
                        $infoSchema = [
                            TextInput::make('backup_name')
                                ->label(__('Backup'))
                                ->default($record->name)
                                ->disabled(),
                        ];

                        if ($isServerBackup || count($users) > 1) {
                            $userOptions = ['__all__' => __('All users')];
                            foreach ($users as $userOption) {
                                $userOptions[$userOption] = $userOption;
                            }
                            $infoSchema[] = Select::make('restore_username')
                                ->label(__('User to Restore'))
                                ->options($userOptions)
                                ->placeholder(__('Select an option'))
                                ->required()
                                ->live()
                                ->helperText(__('Backup contains :count user(s)', ['count' => count($users)]));
                        } else {
                            $infoSchema[] = TextInput::make('restore_username')
                                ->label(__('User'))
                                ->default($selectedUser)
                                ->disabled();
                        }

                        $schema[] = Section::make(__('Backup Information'))
                            ->schema([Grid::make(2)->schema($infoSchema)]);

                        if ($isRemoteBackup) {
                            $schema[] = Section::make(__('Remote Backup'))
                                ->description(__('This backup will be downloaded from the remote destination before restoring.'))
                                ->icon('heroicon-o-cloud-arrow-down')
                                ->iconColor('info');
                        }

                        // Step 2: Restore options with tabs
                        $schema[] = Tabs::make(__('Restore Options'))
                            ->contained()
                            ->visible(fn ($get): bool => ! empty($get('restore_username')))
                            ->tabs([
                                Tab::make(__('Files'))
                                    ->icon('heroicon-o-folder')
                                    ->schema([
                                        Toggle::make('restore_files')
                                            ->label(__('Restore Website Files'))
                                            ->default(false)
                                            ->live(),
                                        Radio::make('files_restore_mode')
                                            ->label(__('Restore Mode'))
                                            ->options([
                                                'domains' => __('Full Domains'),
                                                'files' => __('Specific Files / Folders'),
                                            ])
                                            ->default('domains')
                                            ->inline()
                                            ->live()
                                            ->visible(fn ($get): bool => (bool) $get('restore_files') && $get('restore_username') !== '__all__'),
                                        Select::make('selected_domains')
                                            ->label(__('Select Domains'))
                                            ->multiple()
                                            ->options(function ($get) use ($getContentsForUser): array {
                                                $c = $getContentsForUser($get('restore_username'));
                                                $d = $c['domains'] ?? [];

                                                return ! empty($d) ? array_combine($d, $d) : [];
                                            })
                                            ->placeholder(__('All domains'))
                                            ->helperText(__('Leave empty to restore all domains, or select specific ones'))
                                            ->visible(fn ($get): bool => (bool) $get('restore_files') && $get('files_restore_mode') === 'domains' && $get('restore_username') !== '__all__'),
                                        \Filament\Schemas\Components\Livewire::make(\App\Livewire\BackupSnapshotBrowser::class)
                                            ->data(fn ($get) => [
                                                'backupPath' => $record->remote_path ?? $record->local_path ?? '',
                                                'username' => $get('restore_username') ?? '',
                                                'destinationId' => $record->destination_id ?? 0,
                                            ])
                                            ->key('snapshot-browser')
                                            ->visible(fn ($get): bool => (bool) $get('restore_files') && $get('files_restore_mode') === 'files' && $get('restore_username') !== '__all__'),
                                    ]),
                                Tab::make(__('Databases'))
                                    ->icon('heroicon-o-circle-stack')
                                    ->schema([
                                        Toggle::make('restore_databases')
                                            ->label(__('Restore Databases'))
                                            ->default(false)
                                            ->live(),
                                        Select::make('selected_databases')
                                            ->label(__('Select Databases'))
                                            ->multiple()
                                            ->options(function ($get) use ($getContentsForUser): array {
                                                $c = $getContentsForUser($get('restore_username'));
                                                $d = $c['databases'] ?? [];

                                                return ! empty($d) ? array_combine($d, $d) : [];
                                            })
                                            ->placeholder(__('All databases'))
                                            ->visible(fn ($get): bool => (bool) $get('restore_databases') && $get('restore_username') !== '__all__'),
                                        Toggle::make('restore_mysql_users')
                                            ->label(__('Restore MySQL Users'))
                                            ->default(false)
                                            ->live(),
                                        Select::make('selected_mysql_users')
                                            ->label(__('Select MySQL Users'))
                                            ->multiple()
                                            ->options(function ($get) use ($getContentsForUser): array {
                                                $c = $getContentsForUser($get('restore_username'));
                                                $d = $c['mysql_users'] ?? [];

                                                return ! empty($d) ? array_combine($d, $d) : [];
                                            })
                                            ->placeholder(__('All MySQL users'))
                                            ->visible(fn ($get): bool => (bool) $get('restore_mysql_users') && $get('restore_username') !== '__all__'),
                                    ]),
                                Tab::make(__('Email'))
                                    ->icon('heroicon-o-envelope')
                                    ->schema([
                                        Toggle::make('restore_mailboxes')
                                            ->label(__('Restore Mailboxes'))
                                            ->default(false)
                                            ->live(),
                                        Select::make('selected_mailboxes')
                                            ->label(__('Select Mailboxes'))
                                            ->multiple()
                                            ->options(function ($get) use ($getContentsForUser): array {
                                                $c = $getContentsForUser($get('restore_username'));
                                                $d = $c['mailboxes'] ?? [];

                                                return ! empty($d) ? array_combine($d, $d) : [];
                                            })
                                            ->placeholder(__('All mailboxes'))
                                            ->visible(fn ($get): bool => (bool) $get('restore_mailboxes') && $get('restore_username') !== '__all__'),
                                    ]),
                                Tab::make(__('SSL / DNS'))
                                    ->icon('heroicon-o-shield-check')
                                    ->schema([
                                        Toggle::make('restore_ssl')
                                            ->label(__('Restore SSL Certificates'))
                                            ->default(false)
                                            ->live(),
                                        Select::make('selected_ssl')
                                            ->label(__('Select Certificates'))
                                            ->multiple()
                                            ->options(function ($get) use ($getContentsForUser): array {
                                                $c = $getContentsForUser($get('restore_username'));
                                                $d = $c['ssl_certificates'] ?? [];

                                                return ! empty($d) ? array_combine($d, $d) : [];
                                            })
                                            ->placeholder(__('All certificates'))
                                            ->visible(fn ($get): bool => (bool) $get('restore_ssl') && $get('restore_username') !== '__all__'),
                                        Toggle::make('restore_dns')
                                            ->label(__('Restore DNS Zones'))
                                            ->default(false)
                                            ->live(),
                                        Select::make('selected_dns')
                                            ->label(__('Select Zones'))
                                            ->multiple()
                                            ->options(function ($get) use ($getContentsForUser): array {
                                                $c = $getContentsForUser($get('restore_username'));
                                                $d = $c['dns_zones'] ?? [];

                                                return ! empty($d) ? array_combine($d, $d) : [];
                                            })
                                            ->placeholder(__('All zones'))
                                            ->visible(fn ($get): bool => (bool) $get('restore_dns') && $get('restore_username') !== '__all__'),
                                    ]),
                            ]);

                        return $schema;
                    })
                    ->action(function (array $data, Backup $record): void {
                        $this->executeRestore($record, $data);
                    })
                    ->modalSubmitActionLabel(__('Restore'))
                    ->requiresConfirmation(),
                Action::make('download')
                    ->label(__('Download'))
                    ->icon('heroicon-o-arrow-down-tray')
                    ->color('gray')
                    ->size('sm')
                    ->visible(fn (Backup $record): bool => $record->canDownload())
                    ->url(fn (Backup $record): string => route('filament.admin.pages.backup-download', ['id' => $record->id]))
                    ->openUrlInNewTab(),
                Action::make('delete')
                    ->label(__('Delete'))
                    ->icon('heroicon-o-trash')
                    ->color('danger')
                    ->size('sm')
                    ->requiresConfirmation()
                    ->action(fn (Backup $record) => $this->deleteBackup($record->id)),
            ])
            ->emptyStateHeading(__('No server backups yet'))
            ->emptyStateDescription(__('Click "Create Server Backup" to create your first backup'))
            ->emptyStateIcon('heroicon-o-archive-box')
            ->striped()
            ->poll(fn () => Backup::whereIn('status', ['pending', 'running', 'uploading'])->exists() ? '3s' : null);
    }

    public function getTableRecordKey(Model|array $record): string
    {
        return is_array($record) ? (string) $record['id'] : (string) $record->getKey();
    }

    protected function getHeaderActions(): array
    {
        return [
            Action::make('createServerBackup')
                ->label(__('Create Server Backup'))
                ->icon('heroicon-o-archive-box-arrow-down')
                ->color('primary')
                ->form([
                    TextInput::make('name')
                        ->label(__('Backup Name'))
                        ->default(fn () => __('Server Backup').' '.now()->format('Y-m-d H:i'))
                        ->required(),
                    Select::make('destination_id')
                        ->label(__('Destination'))
                        ->options(fn () => BackupDestination::where('is_server_backup', true)
                            ->where('is_active', true)
                            ->pluck('name', 'id')
                            ->prepend(__('Local Storage'), ''))
                        ->default('')
                        ->live()
                        ->afterStateUpdated(fn ($set, $state) => $set('backup_type', $this->supportsIncremental($state) ? 'incremental' : 'full')),
                    Radio::make('backup_type')
                        ->label(__('Backup Type'))
                        ->options(fn ($get) => $this->supportsIncremental($get('destination_id'))
                            ? [
                                'incremental' => __('Incremental (rsync) - Space-efficient'),
                                'full' => __('Full (tar.gz) - Complete archive'),
                            ]
                            : [
                                'full' => __('Full (tar.gz) - Complete archive'),
                            ])
                        ->default('full')
                        ->required(),
                    TextInput::make('local_path')
                        ->label(__('Local Backup Folder'))
                        ->default('/var/backups/jabali')
                        ->visible(fn ($get) => empty($get('destination_id'))),
                    Section::make(__('Include'))
                        ->schema([
                            Grid::make(2)->schema([
                                Toggle::make('include_files')->label(__('Website Files'))->default(true),
                                Toggle::make('include_databases')->label(__('Databases'))->default(true),
                                Toggle::make('include_mailboxes')->label(__('Mailboxes'))->default(true),
                                Toggle::make('include_dns')->label(__('DNS Records'))->default(true),
                                Toggle::make('include_ssl')->label(__('SSL Certificates'))->default(true),
                            ]),
                        ]),
                    Select::make('users')
                        ->label(__('Users to Backup'))
                        ->multiple()
                        ->options(fn () => User::where('is_admin', false)
                            ->where('is_active', true)
                            ->pluck('username', 'username'))
                        ->placeholder(__('All Users')),
                ])
                ->action(function (array $data) {
                    $this->createServerBackup($data);
                }),

            $this->createUserBackupAction(),

            Action::make('addDestination')
                ->label(__('Add Destination'))
                ->icon('heroicon-o-plus')
                ->color('gray')
                ->form($this->getDestinationForm())
                ->action(function (array $data) {
                    $this->saveDestination($data);
                }),
        ];
    }

    protected function getDestinationForm(): array
    {
        return [
            TextInput::make('name')
                ->label(__('Destination Name'))
                ->required(),
            Select::make('type')
                ->label(__('Type'))
                ->options([
                    'sftp' => __('SFTP Server'),
                    'nfs' => __('NFS Mount'),
                    's3' => __('S3-Compatible Storage'),
                ])
                ->required()
                ->live(),

            Section::make(__('SFTP Settings'))
                ->visible(fn ($get) => $get('type') === 'sftp')
                ->schema([
                    Grid::make(2)->schema([
                        TextInput::make('host')->label(__('Host'))->required(),
                        TextInput::make('port')->label(__('Port'))->numeric()->default(22),
                    ]),
                    TextInput::make('username')->label(__('Username'))->required(),
                    TextInput::make('password')->label(__('Password'))->password(),
                    Textarea::make('private_key')->label(__('Private Key (SSH)'))->rows(4),
                    TextInput::make('path')->label(__('Remote Path'))->default('/backups'),
                ]),

            Section::make(__('NFS Settings'))
                ->visible(fn ($get) => $get('type') === 'nfs')
                ->schema([
                    TextInput::make('server')->label(__('NFS Server'))->required(),
                    TextInput::make('share')->label(__('Share Path'))->required(),
                    TextInput::make('path')->label(__('Sub-directory'))->default(''),
                ]),

            Section::make(__('S3-Compatible Settings'))
                ->visible(fn ($get) => $get('type') === 's3')
                ->schema([
                    TextInput::make('endpoint')->label(__('Endpoint URL')),
                    TextInput::make('bucket')->label(__('Bucket Name'))->required(),
                    Grid::make(2)->schema([
                        TextInput::make('access_key')->label(__('Access Key ID'))->required(),
                        TextInput::make('secret_key')->label(__('Secret Access Key'))->password()->required(),
                    ]),
                    TextInput::make('region')->label(__('Region'))->default('us-east-1'),
                    TextInput::make('path')->label(__('Path Prefix'))->default('backups'),
                ]),

            Toggle::make('is_default')->label(__('Set as Default Destination')),

            FormActions::make([
                Action::make('testConnection')
                    ->label(__('Test Connection'))
                    ->icon('heroicon-o-signal')
                    ->color('gray')
                    ->action(function ($get, $livewire) {
                        $type = $get('type');
                        if (empty($type)) {
                            Notification::make()
                                ->title(__('Select a destination type first'))
                                ->warning()
                                ->send();

                            return;
                        }

                        $config = match ($type) {
                            'sftp' => [
                                'type' => 'sftp',
                                'host' => $get('host') ?? '',
                                'port' => (int) ($get('port') ?? 22),
                                'username' => $get('username') ?? '',
                                'password' => $get('password') ?? '',
                                'private_key' => $get('private_key') ?? '',
                                'path' => $get('path') ?? '/backups',
                            ],
                            'nfs' => [
                                'type' => 'nfs',
                                'server' => $get('server') ?? '',
                                'share' => $get('share') ?? '',
                                'path' => $get('path') ?? '',
                            ],
                            's3' => [
                                'type' => 's3',
                                'endpoint' => $get('endpoint') ?? '',
                                'bucket' => $get('bucket') ?? '',
                                'access_key' => $get('access_key') ?? '',
                                'secret_key' => $get('secret_key') ?? '',
                                'region' => $get('region') ?? 'us-east-1',
                                'path' => $get('path') ?? 'backups',
                            ],
                            default => [],
                        };

                        if (empty($config)) {
                            Notification::make()
                                ->title(__('Invalid destination type'))
                                ->danger()
                                ->send();

                            return;
                        }

                        try {
                            $result = $livewire->agent()->backupTestDestination($config);
                            if ($result['success']) {
                                Notification::make()
                                    ->title(__('Connection successful'))
                                    ->body(__('Connection and write permission verified.'))
                                    ->success()
                                    ->send();
                            } else {
                                Notification::make()
                                    ->title(__('Connection failed'))
                                    ->body($result['error'] ?? __('Could not connect to destination'))
                                    ->danger()
                                    ->send();
                            }
                        } catch (Exception $e) {
                            Notification::make()
                                ->title(__('Connection test failed'))
                                ->body(SafeError::message($e))
                                ->danger()
                                ->send();
                        }
                    }),
            ])->visible(fn ($get) => ! empty($get('type'))),
        ];
    }

    public function saveDestination(array $data): void
    {
        $config = [];
        $type = $data['type'];

        switch ($type) {
            case 'sftp':
                $config = [
                    'host' => $data['host'] ?? '',
                    'port' => (int) ($data['port'] ?? 22),
                    'username' => $data['username'] ?? '',
                    'password' => $data['password'] ?? '',
                    'private_key' => $data['private_key'] ?? '',
                    'path' => $data['path'] ?? '/backups',
                ];
                break;

            case 'nfs':
                $config = [
                    'server' => $data['server'] ?? '',
                    'share' => $data['share'] ?? '',
                    'path' => $data['path'] ?? '',
                ];
                break;

            case 's3':
                $config = [
                    'endpoint' => $data['endpoint'] ?? '',
                    'bucket' => $data['bucket'] ?? '',
                    'access_key' => $data['access_key'] ?? '',
                    'secret_key' => $data['secret_key'] ?? '',
                    'region' => $data['region'] ?? 'us-east-1',
                    'path' => $data['path'] ?? 'backups',
                ];
                break;
        }

        $testConfig = array_merge($config, ['type' => $type]);
        try {
            $result = $this->agent()->backupTestDestination($testConfig);
            if (! $result['success']) {
                Notification::make()
                    ->title(__('Connection failed'))
                    ->body($result['error'] ?? __('Could not connect to destination'))
                    ->danger()
                    ->send();

                return;
            }
        } catch (Exception $e) {
            Notification::make()
                ->title(__('Connection test failed'))
                ->body(SafeError::message($e))
                ->danger()
                ->send();

            return;
        }

        BackupDestination::create([
            'name' => $data['name'],
            'type' => $type,
            'config' => $config,
            'is_server_backup' => true,
            'is_default' => $data['is_default'] ?? false,
            'is_active' => true,
            'last_tested_at' => now(),
            'test_status' => 'success',
        ]);

        Log::info("BackupDestination: Added destination '{$data['name']}' ({$type})");
        Notification::make()->title(__('Destination verified and added'))->success()->send();
        $this->resetTable();
    }

    public function updateDestination(BackupDestination $destination, array $data): void
    {
        $config = [];
        $type = $data['type'];

        switch ($type) {
            case 'sftp':
                $config = [
                    'host' => $data['host'] ?? '',
                    'port' => (int) ($data['port'] ?? 22),
                    'username' => $data['username'] ?? '',
                    'password' => $data['password'] ?? '',
                    'private_key' => $data['private_key'] ?? '',
                    'path' => $data['path'] ?? '/backups',
                ];
                break;

            case 'nfs':
                $config = [
                    'server' => $data['server'] ?? '',
                    'share' => $data['share'] ?? '',
                    'path' => $data['path'] ?? '',
                ];
                break;

            case 's3':
                $config = [
                    'endpoint' => $data['endpoint'] ?? '',
                    'bucket' => $data['bucket'] ?? '',
                    'access_key' => $data['access_key'] ?? '',
                    'secret_key' => $data['secret_key'] ?? '',
                    'region' => $data['region'] ?? 'us-east-1',
                    'path' => $data['path'] ?? 'backups',
                ];
                break;
        }

        $testConfig = array_merge($config, ['type' => $type]);
        try {
            $result = $this->agent()->backupTestDestination($testConfig);
            if (! ($result['success'] ?? false)) {
                Notification::make()
                    ->title(__('Connection failed'))
                    ->body($result['error'] ?? __('Could not connect to destination'))
                    ->danger()
                    ->send();

                return;
            }
        } catch (Exception $e) {
            Notification::make()
                ->title(__('Connection test failed'))
                ->body(SafeError::message($e))
                ->danger()
                ->send();

            return;
        }

        $destination->update([
            'name' => $data['name'],
            'type' => $type,
            'config' => $config,
            'is_default' => $data['is_default'] ?? false,
            'last_tested_at' => now(),
            'test_status' => 'success',
        ]);

        Log::info("BackupDestination: Updated destination '{$data['name']}' ({$type})");
        Notification::make()->title(__('Destination updated and verified'))->success()->send();
        $this->resetTable();
    }

    public function testDestination(int $id): void
    {
        $destination = BackupDestination::find($id);
        if (! $destination) {
            return;
        }

        try {
            $orchestrator = app(\App\Services\Backup\BackupOrchestrator::class);
            $result = $orchestrator->testDestination($destination);

            if ($result['success'] ?? false) {
                Notification::make()->title(__('Connection successful'))->body(__('Connection and write permission verified.'))->success()->send();
            } else {
                Notification::make()->title(__('Connection failed'))->body($result['error'] ?? __('Unknown error'))->danger()->send();
            }
        } catch (Exception $e) {
            Notification::make()->title(__('Test failed'))->body(SafeError::message($e))->danger()->send();
        }

        $this->resetTable();
    }

    public function deleteDestination(int $id): void
    {
        $dest = BackupDestination::find($id);
        $name = $dest?->name ?? "ID $id";
        BackupDestination::where('id', $id)->delete();
        Log::info("BackupDestination: Deleted destination '{$name}'");
        Notification::make()->title(__('Destination deleted'))->success()->send();
        $this->resetTable();
    }

    public function createServerBackup(array $data): void
    {
        $backupType = $data['backup_type'] ?? 'full';
        $timestamp = now()->format('Y-m-d_His');
        $folderName = $timestamp;
        $baseFolder = rtrim($data['local_path'] ?? '/var/backups/jabali', '/');
        $outputPath = "{$baseFolder}/{$folderName}";

        $isIncrementalRemote = $backupType === 'incremental' && ! empty($data['destination_id']);

        if ($isIncrementalRemote) {
            $destination = BackupDestination::find($data['destination_id']);
            if (! $destination || ! in_array($destination->type, ['sftp', 'nfs'])) {
                Notification::make()
                    ->title(__('Invalid destination'))
                    ->body(__('Incremental backups require an SFTP or NFS destination'))
                    ->danger()
                    ->send();

                return;
            }
        }

        // Create backup record with pending status
        $backup = Backup::create([
            'name' => $data['name'],
            'filename' => $folderName,
            'type' => 'server',
            'include_files' => $data['include_files'] ?? true,
            'include_databases' => $data['include_databases'] ?? true,
            'include_mailboxes' => $data['include_mailboxes'] ?? true,
            'include_dns' => $data['include_dns'] ?? true,
            'include_ssl' => $data['include_ssl'] ?? true,
            'users' => ! empty($data['users']) ? $data['users'] : null,
            'destination_id' => ! empty($data['destination_id']) ? $data['destination_id'] : null,
            'schedule_id' => $data['schedule_id'] ?? null,
            'status' => 'pending',
            'local_path' => $isIncrementalRemote ? null : $outputPath,
            'metadata' => ['backup_type' => $backupType],
        ]);

        // Dispatch job to run backup in background
        \App\Jobs\RunServerBackup::dispatch($backup->id);

        // Show notification and refresh table
        Notification::make()
            ->title(__('Backup started'))
            ->body(__('The backup is running in the background. The status will update automatically.'))
            ->info()
            ->send();

        $this->resetTable();
    }

    protected function uploadToRemote(Backup $backup, bool $keepLocal = false): bool
    {
        $orchestrator = app(\App\Services\Backup\BackupOrchestrator::class);

        return $orchestrator->uploadToRemote($backup, $keepLocal);
    }

    public function deleteBackup(int $id): void
    {
        $backup = Backup::find($id);
        if (! $backup) {
            return;
        }

        try {
            $orchestrator = app(\App\Services\Backup\BackupOrchestrator::class);
            $orchestrator->deleteBackup($backup);
            Notification::make()->title(__('Backup deleted'))->success()->send();
        } catch (Exception $e) {
            Notification::make()
                ->title(__('Failed to delete backup'))
                ->body(SafeError::message($e))
                ->danger()
                ->send();

            return;
        }

        $this->resetTable();
    }

    public function addScheduleAction(): Action
    {
        return Action::make('addSchedule')
            ->label(__('Add Schedule'))
            ->icon('heroicon-o-clock')
            ->color('primary')
            ->form([
                TextInput::make('name')
                    ->label(__('Schedule Name'))
                    ->required(),
                Select::make('destination_id')
                    ->label(__('Destination'))
                    ->options(fn () => BackupDestination::where('is_server_backup', true)
                        ->where('is_active', true)
                        ->pluck('name', 'id')
                        ->prepend(__('Local Storage'), ''))
                    ->default('')
                    ->live()
                    ->afterStateUpdated(fn ($set, $state) => $set('backup_type', $this->supportsIncremental($state) ? 'incremental' : 'full')),
                Radio::make('backup_type')
                    ->label(__('Backup Type'))
                    ->options(fn ($get) => $this->supportsIncremental($get('destination_id'))
                        ? [
                            'incremental' => __('Incremental (rsync)'),
                            'full' => __('Full (tar.gz)'),
                        ]
                        : [
                            'full' => __('Full (tar.gz)'),
                        ])
                    ->default('full')
                    ->required(),
                Select::make('frequency')
                    ->label(__('Frequency'))
                    ->options([
                        'hourly' => __('Hourly'),
                        'daily' => __('Daily'),
                        'weekly' => __('Weekly'),
                        'monthly' => __('Monthly'),
                    ])
                    ->required()
                    ->live(),
                TextInput::make('time')
                    ->label(__('Time (HH:MM)'))
                    ->default('02:00')
                    ->visible(fn ($get) => in_array($get('frequency'), ['daily', 'weekly', 'monthly'])),
                Select::make('day_of_week')
                    ->label(__('Day of Week'))
                    ->options([
                        0 => __('Sunday'), 1 => __('Monday'), 2 => __('Tuesday'),
                        3 => __('Wednesday'), 4 => __('Thursday'), 5 => __('Friday'), 6 => __('Saturday'),
                    ])
                    ->visible(fn ($get) => $get('frequency') === 'weekly'),
                Select::make('day_of_month')
                    ->label(__('Day of Month'))
                    ->options(array_combine(range(1, 28), range(1, 28)))
                    ->visible(fn ($get) => $get('frequency') === 'monthly'),
                TextInput::make('retention_count')
                    ->label(__('Keep Last N Backups'))
                    ->numeric()
                    ->default(7),
                Section::make(__('Include'))
                    ->schema([
                        Grid::make(2)->schema([
                            Toggle::make('include_files')->label(__('Website Files'))->default(true),
                            Toggle::make('include_databases')->label(__('Databases'))->default(true),
                            Toggle::make('include_mailboxes')->label(__('Mailboxes'))->default(true),
                            Toggle::make('include_dns')->label(__('DNS Records'))->default(true),
                        ]),
                    ]),
            ])
            ->action(function (array $data) {
                $schedule = BackupSchedule::create([
                    'name' => $data['name'],
                    'is_server_backup' => true,
                    'is_active' => true,
                    'frequency' => $data['frequency'],
                    'time' => $data['time'] ?? '02:00',
                    'day_of_week' => $data['day_of_week'] ?? null,
                    'day_of_month' => $data['day_of_month'] ?? null,
                    'destination_id' => ! empty($data['destination_id']) ? $data['destination_id'] : null,
                    'retention_count' => $data['retention_count'] ?? 7,
                    'include_files' => $data['include_files'] ?? true,
                    'include_databases' => $data['include_databases'] ?? true,
                    'include_mailboxes' => $data['include_mailboxes'] ?? true,
                    'include_dns' => $data['include_dns'] ?? true,
                    'include_ssl' => $data['include_ssl'] ?? true,
                    'metadata' => ['backup_type' => $data['backup_type'] ?? 'full'],
                ]);

                $schedule->calculateNextRun();
                $schedule->save();

                Notification::make()->title(__('Schedule created'))->success()->send();
                $this->resetTable();
            });
    }

    public function toggleSchedule(int $id): void
    {
        $schedule = BackupSchedule::find($id);
        if (! $schedule) {
            return;
        }

        $schedule->update(['is_active' => ! $schedule->is_active]);

        if ($schedule->is_active) {
            $schedule->calculateNextRun();
            $schedule->save();
        }

        Notification::make()->title($schedule->is_active ? __('Schedule enabled') : __('Schedule disabled'))->success()->send();
        $this->resetTable();
    }

    public function deleteSchedule(int $id): void
    {
        BackupSchedule::where('id', $id)->delete();
        Notification::make()->title(__('Schedule deleted'))->success()->send();
        $this->resetTable();
    }

    public function editScheduleAction(): Action
    {
        return Action::make('editSchedule')
            ->label(__('Edit Schedule'))
            ->icon('heroicon-o-pencil')
            ->color('gray')
            ->fillForm(function (array $arguments): array {
                $schedule = BackupSchedule::find($arguments['id']);
                if (! $schedule) {
                    return [];
                }

                return [
                    'name' => $schedule->name,
                    'backup_type' => $schedule->metadata['backup_type'] ?? 'full',
                    'frequency' => $schedule->frequency,
                    'time' => $schedule->time,
                    'day_of_week' => $schedule->day_of_week,
                    'day_of_month' => $schedule->day_of_month,
                    'destination_id' => $schedule->destination_id ?? '',
                    'retention_count' => $schedule->retention_count,
                    'include_files' => $schedule->include_files,
                    'include_databases' => $schedule->include_databases,
                    'include_mailboxes' => $schedule->include_mailboxes,
                    'include_dns' => $schedule->include_dns,
                    'include_ssl' => $schedule->include_ssl ?? true,
                ];
            })
            ->form([
                TextInput::make('name')->label(__('Schedule Name'))->required(),
                Select::make('destination_id')
                    ->label(__('Destination'))
                    ->options(fn () => BackupDestination::where('is_server_backup', true)
                        ->where('is_active', true)
                        ->pluck('name', 'id')
                        ->prepend(__('Local Storage'), ''))
                    ->default('')
                    ->live(),
                Radio::make('backup_type')
                    ->label(__('Backup Type'))
                    ->options(fn ($get) => $this->supportsIncremental($get('destination_id'))
                        ? ['incremental' => __('Incremental'), 'full' => __('Full')]
                        : ['full' => __('Full')])
                    ->required(),
                Select::make('frequency')
                    ->label(__('Frequency'))
                    ->options(['hourly' => __('Hourly'), 'daily' => __('Daily'), 'weekly' => __('Weekly'), 'monthly' => __('Monthly')])
                    ->required()
                    ->live(),
                TextInput::make('time')->label(__('Time (HH:MM)'))->visible(fn ($get) => in_array($get('frequency'), ['daily', 'weekly', 'monthly'])),
                Select::make('day_of_week')
                    ->label(__('Day of Week'))
                    ->options([0 => __('Sunday'), 1 => __('Monday'), 2 => __('Tuesday'), 3 => __('Wednesday'), 4 => __('Thursday'), 5 => __('Friday'), 6 => __('Saturday')])
                    ->visible(fn ($get) => $get('frequency') === 'weekly'),
                Select::make('day_of_month')
                    ->label(__('Day of Month'))
                    ->options(array_combine(range(1, 28), range(1, 28)))
                    ->visible(fn ($get) => $get('frequency') === 'monthly'),
                TextInput::make('retention_count')->label(__('Keep Last N Backups'))->numeric(),
                Section::make(__('Include'))
                    ->schema([
                        Grid::make(2)->schema([
                            Toggle::make('include_files')->label(__('Website Files')),
                            Toggle::make('include_databases')->label(__('Databases')),
                            Toggle::make('include_mailboxes')->label(__('Mailboxes')),
                            Toggle::make('include_dns')->label(__('DNS Records')),
                        ]),
                    ]),
            ])
            ->action(function (array $data, array $arguments) {
                $schedule = BackupSchedule::find($arguments['id']);
                if (! $schedule) {
                    return;
                }

                $schedule->update([
                    'name' => $data['name'],
                    'frequency' => $data['frequency'],
                    'time' => $data['time'] ?? '02:00',
                    'day_of_week' => $data['day_of_week'] ?? null,
                    'day_of_month' => $data['day_of_month'] ?? null,
                    'destination_id' => ! empty($data['destination_id']) ? $data['destination_id'] : null,
                    'retention_count' => $data['retention_count'] ?? 7,
                    'include_files' => $data['include_files'] ?? true,
                    'include_databases' => $data['include_databases'] ?? true,
                    'include_mailboxes' => $data['include_mailboxes'] ?? true,
                    'include_dns' => $data['include_dns'] ?? true,
                    'include_ssl' => $data['include_ssl'] ?? true,
                    'metadata' => array_merge($schedule->metadata ?? [], ['backup_type' => $data['backup_type'] ?? 'full']),
                ]);

                $schedule->calculateNextRun();
                $schedule->save();

                Notification::make()->title(__('Schedule updated'))->success()->send();
                $this->resetTable();
            });
    }

    public function runScheduleNow(int $id): void
    {
        $schedule = BackupSchedule::find($id);
        if (! $schedule) {
            return;
        }

        $runningBackup = Backup::where('schedule_id', $id)->running()->first();
        if ($runningBackup) {
            Notification::make()->title(__('Backup already running'))->warning()->send();

            return;
        }

        $this->createServerBackup([
            'name' => $schedule->name.' - '.__('Manual Run').' '.now()->format('Y-m-d H:i'),
            'backup_type' => $schedule->metadata['backup_type'] ?? 'full',
            'destination_id' => $schedule->destination_id,
            'schedule_id' => $schedule->id,
            'include_files' => $schedule->include_files,
            'include_databases' => $schedule->include_databases,
            'include_mailboxes' => $schedule->include_mailboxes,
            'include_dns' => $schedule->include_dns,
            'users' => $schedule->users,
        ]);
    }

    public function stopScheduleBackup(int $id): void
    {
        $backup = Backup::where('schedule_id', $id)->running()->first();
        if ($backup) {
            $backup->update([
                'status' => 'failed',
                'error_message' => __('Cancelled by user'),
                'completed_at' => now(),
            ]);
            Notification::make()->title(__('Backup cancelled'))->success()->send();
            $this->resetTable();
        }
    }

    public function createUserBackupAction(): Action
    {
        return Action::make('createUserBackup')
            ->label(__('Backup User'))
            ->icon('heroicon-o-user')
            ->color('gray')
            ->form([
                Select::make('user_id')
                    ->label(__('User'))
                    ->options(fn () => User::where('is_admin', false)
                        ->where('is_active', true)
                        ->pluck('username', 'id'))
                    ->required()
                    ->searchable(),
                Section::make(__('Include'))
                    ->schema([
                        Grid::make(2)->schema([
                            Toggle::make('include_files')->label(__('Website Files'))->default(true),
                            Toggle::make('include_databases')->label(__('Databases'))->default(true),
                            Toggle::make('include_mailboxes')->label(__('Mailboxes'))->default(true),
                            Toggle::make('include_dns')->label(__('DNS Records'))->default(true),
                        ]),
                    ]),
            ])
            ->action(function (array $data) {
                $user = User::find($data['user_id']);
                if (! $user) {
                    Notification::make()->title(__('User not found'))->danger()->send();

                    return;
                }

                $timestamp = now()->format('Y-m-d_His');
                $filename = "backup_{$timestamp}.tar.gz";
                $outputPath = "/home/{$user->username}/backups/{$filename}";

                $backup = Backup::create([
                    'user_id' => $user->id,
                    'name' => "{$user->username} ".__('Backup').' '.now()->format('Y-m-d H:i'),
                    'filename' => $filename,
                    'type' => 'full',
                    'include_files' => $data['include_files'] ?? true,
                    'include_databases' => $data['include_databases'] ?? true,
                    'include_mailboxes' => $data['include_mailboxes'] ?? true,
                    'include_dns' => $data['include_dns'] ?? true,
                    'include_ssl' => $data['include_ssl'] ?? true,
                    'status' => 'pending',
                    'local_path' => $outputPath,
                    'metadata' => ['backup_type' => 'full'],
                ]);

                try {
                    $backup->update(['status' => 'running', 'started_at' => now()]);

                    $result = $this->agent()->backupCreate($user->username, $outputPath, [
                        'backup_type' => 'full',
                        'include_files' => $data['include_files'] ?? true,
                        'include_databases' => $data['include_databases'] ?? true,
                        'include_mailboxes' => $data['include_mailboxes'] ?? true,
                        'include_dns' => $data['include_dns'] ?? true,
                        'include_ssl' => $data['include_ssl'] ?? true,
                    ]);

                    if ($result['success']) {
                        $backup->update([
                            'status' => 'completed',
                            'completed_at' => now(),
                            'size_bytes' => $result['size'] ?? 0,
                            'checksum' => $result['checksum'] ?? null,
                            'domains' => $result['domains'] ?? null,
                            'databases' => $result['databases'] ?? null,
                            'mailboxes' => $result['mailboxes'] ?? null,
                        ]);
                        Notification::make()->title(__('Backup created for :username', ['username' => $user->username]))->success()->send();
                    } else {
                        throw new Exception($result['error'] ?? __('Backup failed'));
                    }
                } catch (Exception $e) {
                    $backup->update([
                        'status' => 'failed',
                        'completed_at' => now(),
                        'error_message' => $e->getMessage(),
                    ]);
                    Notification::make()->title(__('Backup failed'))->body(SafeError::message($e))->danger()->send();
                }

                $this->resetTable();
            });
    }

    protected function executeRestore(Backup $backup, array $data): void
    {
        // Use username from form (allows selecting user for server backups)
        $username = $data['restore_username'] ?? '';

        if ($username === '__all__') {
            $manifest = $this->getBackupManifest($backup);
            $usernames = $manifest['users'] ?? $backup->users ?? [];
            if (empty($usernames)) {
                $usernames = array_filter([$manifest['username'] ?? '']);
            }

            if (empty($usernames)) {
                Notification::make()->title(__('Cannot determine users for this backup'))->danger()->send();

                return;
            }

            foreach ($usernames as $restoreUser) {
                $data['restore_username'] = $restoreUser;
                $this->executeRestore($backup, $data);
            }

            return;
        }

        if (empty($username)) {
            $manifest = $this->getBackupManifest($backup);
            $username = $manifest['username'] ?? ($backup->users[0] ?? '');
        }

        if (empty($username)) {
            Notification::make()->title(__('Cannot determine user for this backup'))->danger()->send();

            return;
        }

        // Prepare backup path
        $backupPath = $backup->local_path;
        $tempDownloadPath = null;

        // For remote backups, download first
        if ((! $backupPath || ! file_exists($backupPath)) && $backup->remote_path && $backup->destination) {
            $destination = $backup->destination;

            if (! $destination) {
                Notification::make()->title(__('Backup destination not found'))->danger()->send();

                return;
            }

            // Create temp directory for download
            $tempDownloadPath = sys_get_temp_dir().'/jabali_restore_download_'.uniqid();
            mkdir($tempDownloadPath, 0755, true);

            // For incremental backups, we need to download the specific user's directory
            $remotePath = $backup->remote_path;

            // If it's a server backup with per-user directories, construct the user-specific path
            if (str_contains($remotePath, '/') && ! str_ends_with($remotePath, '.tar.gz')) {
                // Incremental backup - download the user's directory
                $userRemotePath = rtrim($remotePath, '/').'/'.$username;
            } else {
                $userRemotePath = $remotePath;
            }

            Notification::make()
                ->title(__('Downloading backup'))
                ->body(__('Downloading from remote destination...'))
                ->info()
                ->send();

            try {
                $downloadResult = $this->agent()->send('backup.download_remote', [
                    'remote_path' => $userRemotePath,
                    'local_path' => $tempDownloadPath,
                    'destination' => array_merge(
                        $destination->config ?? [],
                        ['type' => $destination->type]
                    ),
                ]);

                if (! ($downloadResult['success'] ?? false)) {
                    throw new Exception($downloadResult['error'] ?? __('Failed to download backup'));
                }

                $backupPath = $tempDownloadPath;
            } catch (Exception $e) {
                // Cleanup temp directory on failure
                if (is_dir($tempDownloadPath)) {
                    try {
                        File::deleteDirectory($tempDownloadPath);
                    } catch (\Throwable) {
                        try {
                            $this->agent()->backupDeleteServer($tempDownloadPath);
                        } catch (\Throwable) {
                        }
                    }
                }
                Notification::make()
                    ->title(__('Download failed'))
                    ->body(SafeError::message($e))
                    ->danger()
                    ->send();

                return;
            }
        }

        if (! $backupPath || ! file_exists($backupPath)) {
            Notification::make()->title(__('Backup file not found'))->danger()->send();

            return;
        }

        try {
            $result = $this->agent()->send('backup.restore', [
                'username' => $username,
                'backup_path' => $backupPath,
                'restore_files' => $data['restore_files'] ?? false,
                'restore_databases' => $data['restore_databases'] ?? false,
                'restore_mailboxes' => $data['restore_mailboxes'] ?? false,
                'restore_dns' => $data['restore_dns'] ?? false,
                'restore_ssl' => $data['restore_ssl'] ?? false,
                'selected_domains' => ! empty($data['selected_domains']) ? $data['selected_domains'] : null,
                'selected_databases' => ! empty($data['selected_databases']) ? $data['selected_databases'] : null,
                'selected_mailboxes' => ! empty($data['selected_mailboxes']) ? $data['selected_mailboxes'] : null,
            ]);

            // Cleanup temp download if used (root-owned, fallback to agent)
            if ($tempDownloadPath && is_dir($tempDownloadPath)) {
                try {
                    File::deleteDirectory($tempDownloadPath);
                } catch (\Throwable) {
                    try {
                        $this->agent()->backupDeleteServer($tempDownloadPath);
                    } catch (\Throwable) {
                    }
                }
            }

            if ($result['success'] ?? false) {
                $restored = $result['restored'] ?? [];
                $summary = [];

                if (! empty($restored['files'])) {
                    $summary[] = count($restored['files']).' '.__('domain(s)');
                }
                if (! empty($restored['databases'])) {
                    $summary[] = count($restored['databases']).' '.__('database(s)');
                }
                if (! empty($restored['mailboxes'])) {
                    $summary[] = count($restored['mailboxes']).' '.__('mailbox(es)');
                }
                if (! empty($restored['ssl_certificates'])) {
                    $summary[] = count($restored['ssl_certificates']).' '.__('SSL cert(s)');
                }
                if (! empty($restored['dns_zones'])) {
                    $summary[] = count($restored['dns_zones']).' '.__('DNS zone(s)');
                }
                if ($restored['mysql_users'] ?? false) {
                    $summary[] = __('MySQL users');
                }

                Notification::make()
                    ->title(__('Restore completed'))
                    ->body(! empty($summary) ? __('Restored: :items', ['items' => implode(', ', $summary)]) : __('Nothing was restored'))
                    ->success()
                    ->send();
            } else {
                throw new Exception($result['error'] ?? __('Restore failed'));
            }
        } catch (Exception $e) {
            // Cleanup temp download on failure (root-owned, fallback to agent)
            if ($tempDownloadPath && is_dir($tempDownloadPath)) {
                try {
                    File::deleteDirectory($tempDownloadPath);
                } catch (\Throwable) {
                    try {
                        $this->agent()->backupDeleteServer($tempDownloadPath);
                    } catch (\Throwable) {
                    }
                }
            }
            Notification::make()
                ->title(__('Restore failed'))
                ->body(SafeError::message($e))
                ->danger()
                ->send();
        }
    }

    protected function getBackupManifest(Backup $backup, ?string $forUser = null): array
    {
        $orchestrator = app(\App\Services\Backup\BackupOrchestrator::class);

        return $orchestrator->getBackupManifest($backup, $forUser);
    }

    /**
     * Get the system timezone for display purposes.
     * Laravel uses UTC internally but we display times in server's local timezone.
     */
    protected function getSystemTimezone(): string
    {
        static $timezone = null;
        if ($timezone === null) {
            $timezone = ServerFacts::timezone();
        }

        return $timezone;
    }
}
