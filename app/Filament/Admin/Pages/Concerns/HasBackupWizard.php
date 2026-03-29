<?php

declare(strict_types=1);

namespace App\Filament\Admin\Pages\Concerns;

use App\Models\BackupDestination;
use App\Models\BackupSchedule;
use App\Models\DnsSetting;
use App\Services\Agent\AgentClient;
use App\Services\Backup\BackupOrchestrator;
use App\Support\SafeError;
use Exception;
use Filament\Actions\Action;
use Filament\Forms\Components\Placeholder;
use Filament\Forms\Components\Select;
use Filament\Forms\Components\Textarea;
use Filament\Forms\Components\TextInput;
use Filament\Forms\Components\Toggle;
use Filament\Notifications\Notification;
use Filament\Schemas\Components\Grid;
use Filament\Schemas\Components\Wizard\Step;
use Illuminate\Support\HtmlString;

trait HasBackupWizard
{
    protected function mountBackupWizard(): void
    {
        if (! DnsSetting::get('backup_wizard_completed', false)) {
            $this->defaultAction = 'backupWizard';
        }
    }

    protected function backupWizardAction(): Action
    {
        return Action::make('backupWizard')
            ->label(__('Backup Setup'))
            ->icon('heroicon-o-shield-check')
            ->modalHeading(__('Backup Setup'))
            ->modalDescription(__('Set up encrypted backups for your server.'))
            ->modalWidth('2xl')
            ->modalCancelActionLabel(__('Skip for now'))
            ->fillForm(function (): array {
                $currentPassword = '';
                try {
                    $result = app(AgentClient::class)->send('backup.get_password', []);
                    $currentPassword = $result['password'] ?? '';
                } catch (\Throwable) {
                }

                if (empty($currentPassword)) {
                    $currentPassword = bin2hex(random_bytes(16));
                }

                return [
                    'encryption_password' => $currentPassword,
                    'add_remote' => false,
                    'dest_type' => 'sftp',
                    'schedule_name' => __('Daily Server Backup'),
                    'frequency' => 'daily',
                    'time' => '03:00',
                    'retention_count' => 7,
                    'include_files' => true,
                    'include_databases' => true,
                    'include_mailboxes' => true,
                ];
            })
            ->steps([
                Step::make(__('Encryption'))
                    ->icon('heroicon-o-lock-closed')
                    ->description(__('Secure your backups'))
                    ->schema([
                        Placeholder::make('encryption_info')
                            ->content(new HtmlString(
                                '<div class="rounded-lg border border-warning-500/30 bg-warning-50 p-4 text-sm text-warning-800 dark:bg-warning-950/30 dark:text-warning-400">'.
                                '<p class="font-semibold">'.__('All backups are encrypted with Restic for security.').'</p>'.
                                '<p class="mt-2">'.__('This password is the only way to access your backup data. If you lose it, your backups will be permanently unrecoverable.').'</p>'.
                                '<p class="mt-2 font-semibold">'.__('Save this password in a secure location now.').'</p>'.
                                '</div>'
                            )),
                        TextInput::make('encryption_password')
                            ->label(__('Encryption Password'))
                            ->password()
                            ->revealable()
                            ->required()
                            ->minLength(12)
                            ->helperText(__('Auto-generated. You may change it, but save it somewhere safe.')),
                    ]),

                Step::make(__('Destination'))
                    ->icon('heroicon-o-cloud-arrow-up')
                    ->description(__('Where to store backups'))
                    ->schema([
                        Placeholder::make('destination_info')
                            ->content(__('Backups are stored locally by default. You can optionally add a remote destination for off-site protection.')),
                        Toggle::make('add_remote')
                            ->label(__('Add a remote destination'))
                            ->live(),
                        Select::make('dest_type')
                            ->label(__('Type'))
                            ->options([
                                'sftp' => __('SFTP / SSH'),
                                's3' => __('Amazon S3'),
                                'b2' => __('Backblaze B2'),
                                'wasabi' => __('Wasabi'),
                                'minio' => __('MinIO / S3-Compatible'),
                                'rest' => __('Restic REST Server'),
                                'local' => __('Local Path'),
                            ])
                            ->visible(fn ($get) => $get('add_remote'))
                            ->required(fn ($get) => $get('add_remote'))
                            ->live(),
                        TextInput::make('dest_name')
                            ->label(__('Name'))
                            ->placeholder(__('My Backup Server'))
                            ->visible(fn ($get) => $get('add_remote'))
                            ->required(fn ($get) => $get('add_remote')),
                        // SFTP fields
                        Grid::make(2)
                            ->schema([
                                TextInput::make('host')
                                    ->label(__('Host'))
                                    ->required(fn ($get) => $get('dest_type') === 'sftp'),
                                TextInput::make('port')
                                    ->label(__('Port'))
                                    ->numeric()
                                    ->default(22)
                                    ->minValue(1)
                                    ->maxValue(65535),
                            ])
                            ->visible(fn ($get) => $get('add_remote') && $get('dest_type') === 'sftp'),
                        TextInput::make('username')
                            ->label(__('Username'))
                            ->visible(fn ($get) => $get('add_remote') && $get('dest_type') === 'sftp')
                            ->required(fn ($get) => $get('add_remote') && $get('dest_type') === 'sftp'),
                        TextInput::make('password')
                            ->label(__('Password'))
                            ->password()
                            ->visible(fn ($get) => $get('add_remote') && $get('dest_type') === 'sftp'),
                        Textarea::make('private_key')
                            ->label(__('SSH Private Key'))
                            ->rows(3)
                            ->visible(fn ($get) => $get('add_remote') && $get('dest_type') === 'sftp')
                            ->helperText(__('Optional, alternative to password')),
                        TextInput::make('path')
                            ->label(__('Remote Path'))
                            ->default('/backups')
                            ->visible(fn ($get) => $get('add_remote') && in_array($get('dest_type'), ['sftp', 'local'])),
                        // S3-compatible fields
                        TextInput::make('endpoint')
                            ->label(__('Endpoint'))
                            ->visible(fn ($get) => $get('add_remote') && in_array($get('dest_type'), ['s3', 'b2', 'wasabi', 'minio']))
                            ->required(fn ($get) => $get('add_remote') && in_array($get('dest_type'), ['s3', 'b2', 'wasabi', 'minio'])),
                        TextInput::make('bucket')
                            ->label(__('Bucket'))
                            ->visible(fn ($get) => $get('add_remote') && in_array($get('dest_type'), ['s3', 'b2', 'wasabi', 'minio']))
                            ->required(fn ($get) => $get('add_remote') && in_array($get('dest_type'), ['s3', 'b2', 'wasabi', 'minio'])),
                        TextInput::make('access_key')
                            ->label(__('Access Key'))
                            ->visible(fn ($get) => $get('add_remote') && in_array($get('dest_type'), ['s3', 'b2', 'wasabi', 'minio']))
                            ->required(fn ($get) => $get('add_remote') && in_array($get('dest_type'), ['s3', 'b2', 'wasabi', 'minio'])),
                        TextInput::make('secret_key')
                            ->label(__('Secret Key'))
                            ->password()
                            ->visible(fn ($get) => $get('add_remote') && in_array($get('dest_type'), ['s3', 'b2', 'wasabi', 'minio']))
                            ->required(fn ($get) => $get('add_remote') && in_array($get('dest_type'), ['s3', 'b2', 'wasabi', 'minio'])),
                        // REST server
                        TextInput::make('rest_url')
                            ->label(__('REST Server URL'))
                            ->placeholder('https://backup.example.com:8000')
                            ->visible(fn ($get) => $get('add_remote') && $get('dest_type') === 'rest')
                            ->required(fn ($get) => $get('add_remote') && $get('dest_type') === 'rest'),
                        TextInput::make('rest_username')
                            ->label(__('Username'))
                            ->visible(fn ($get) => $get('add_remote') && $get('dest_type') === 'rest'),
                        TextInput::make('rest_password')
                            ->label(__('Password'))
                            ->password()
                            ->visible(fn ($get) => $get('add_remote') && $get('dest_type') === 'rest'),
                    ]),

                Step::make(__('Schedule'))
                    ->icon('heroicon-o-clock')
                    ->description(__('When to run backups'))
                    ->schema([
                        TextInput::make('schedule_name')
                            ->label(__('Schedule Name'))
                            ->required(),
                        Grid::make(2)->schema([
                            Select::make('frequency')
                                ->label(__('Frequency'))
                                ->options([
                                    'daily' => __('Daily'),
                                    'weekly' => __('Weekly'),
                                    'monthly' => __('Monthly'),
                                ])
                                ->required(),
                            TextInput::make('time')
                                ->label(__('Time (HH:MM)'))
                                ->placeholder('03:00')
                                ->required()
                                ->regex('/^(?:[01]\d|2[0-3]):[0-5]\d$/'),
                        ]),
                        TextInput::make('retention_count')
                            ->label(__('Keep last N backups'))
                            ->numeric()
                            ->minValue(1)
                            ->maxValue(365),
                        Grid::make(3)->schema([
                            Toggle::make('include_files')->label(__('Files'))->default(true),
                            Toggle::make('include_databases')->label(__('Databases'))->default(true),
                            Toggle::make('include_mailboxes')->label(__('Mailboxes'))->default(true),
                        ]),
                        Toggle::make('never_show_again')
                            ->label(__('Don\'t show this wizard again'))
                            ->default(true)
                            ->helperText(__('You can always access backup settings from the header buttons.')),
                    ]),
            ])
            ->action(function (array $data): void {
                // 1. Save encryption password
                try {
                    app(AgentClient::class)->send('backup.set_password', [
                        'password' => $data['encryption_password'],
                    ]);
                } catch (Exception $e) {
                    Notification::make()
                        ->title(__('Failed to set encryption password'))
                        ->body(SafeError::message($e))
                        ->danger()
                        ->send();

                    return;
                }

                // 2. Create remote destination if requested
                $destinationId = null;
                if (! empty($data['add_remote']) && ! empty($data['dest_type'])) {
                    try {
                        $config = $this->buildConfig($data['dest_type'], $data);
                        $dest = BackupDestination::create([
                            'name' => $data['dest_name'] ?? __('Remote Backup'),
                            'type' => $data['dest_type'],
                            'config' => $config,
                            'is_server_backup' => true,
                            'is_active' => true,
                        ]);

                        app(BackupOrchestrator::class)->testDestination($dest);
                        $destinationId = $dest->id;

                        if ($dest->fresh()->test_status === 'failed') {
                            Notification::make()
                                ->title(__('Destination added but connection failed'))
                                ->body($dest->test_message ?? __('Check credentials'))
                                ->warning()
                                ->send();
                        }
                    } catch (Exception $e) {
                        Notification::make()
                            ->title(__('Failed to add destination'))
                            ->body(SafeError::message($e))
                            ->warning()
                            ->send();
                    }
                }

                // 3. Create backup schedule
                $timeParts = explode(':', $data['time'] ?? '03:00');
                $hour = (int) ($timeParts[0] ?? 3);
                $minute = (int) ($timeParts[1] ?? 0);

                BackupSchedule::create([
                    'name' => $data['schedule_name'] ?? __('Daily Server Backup'),
                    'is_active' => true,
                    'is_server_backup' => true,
                    'frequency' => $data['frequency'] ?? 'daily',
                    'time' => sprintf('%02d:%02d', $hour, $minute),
                    'destination_id' => $destinationId,
                    'retention_count' => (int) ($data['retention_count'] ?? 7),
                    'include_files' => $data['include_files'] ?? true,
                    'include_databases' => $data['include_databases'] ?? true,
                    'include_mailboxes' => $data['include_mailboxes'] ?? true,
                    'include_dns' => true,
                    'include_ssl' => true,
                    'next_run_at' => now()->setTime($hour, $minute)->addDay(),
                ]);

                // 4. Mark wizard as completed if user chose "don't show again"
                if (! empty($data['never_show_again'])) {
                    DnsSetting::set('backup_wizard_completed', '1');
                    DnsSetting::clearCache();
                }

                Notification::make()
                    ->title(__('Backup setup complete'))
                    ->body(__('Your encryption password has been saved and a backup schedule has been created.'))
                    ->success()
                    ->send();
            });
    }
}
