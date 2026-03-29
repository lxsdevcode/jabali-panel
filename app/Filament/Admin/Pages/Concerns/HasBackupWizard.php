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
use Filament\Schemas\Components\Wizard;
use Filament\Schemas\Components\Wizard\Step;
use Illuminate\Support\HtmlString;
use Livewire\Attributes\Locked;

trait HasBackupWizard
{
    #[Locked]
    public ?int $wizardDestinationId = null;

    protected function mountBackupWizard(): void
    {
        if (! DnsSetting::get('backup_wizard_completed', false)) {
            $this->defaultAction = 'backupWizard';
        }
    }

    public function dismissBackupWizard(): void
    {
        DnsSetting::set('backup_wizard_completed', '1');
        DnsSetting::clearCache();
        $this->redirect(static::getUrl());
    }

    protected function backupWizardAction(): Action
    {
        return Action::make('backupWizard')
            ->label(__('Backup Setup'))
            ->icon('heroicon-o-shield-check')
            ->modalHeading(__('Backup Setup'))
            ->modalDescription(__('Set up encrypted backups for your server.'))
            ->modalWidth('4xl')
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
            ->modalSubmitAction(false)
            ->schema([
                Wizard::make([
                    Step::make(__('Encryption'))
                        ->icon('heroicon-o-lock-closed')
                        ->description(__('Secure your backups'))
                        ->schema([
                            Placeholder::make('encryption_info')
                                ->hiddenLabel()
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
                            Placeholder::make('dismiss_wizard')
                                ->hiddenLabel()
                                ->content(new HtmlString(
                                    '<button type="button" wire:click="dismissBackupWizard" class="text-sm text-gray-500 underline hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200">'.__('Don\'t show this wizard again').'</button>'
                                )),
                        ]),

                    Step::make(__('Destination'))
                        ->icon('heroicon-o-cloud-arrow-up')
                        ->description(__('Where to store backups'))
                        ->schema([
                            Placeholder::make('destination_info')
                                ->hiddenLabel()
                                ->content(__('Add a remote backup destination for off-site protection.')),
                            Select::make('dest_type')
                                ->label(__('Type'))
                                ->options([
                                    'sftp' => __('SFTP / SSH'),
                                    's3' => __('Amazon S3'),
                                    'b2' => __('Backblaze B2'),
                                    'wasabi' => __('Wasabi'),
                                    'minio' => __('MinIO / S3-Compatible'),
                                    'rest' => __('Restic REST Server'),
                                ])
                                ->required()
                                ->live(),
                            TextInput::make('dest_name')
                                ->label(__('Name'))
                                ->placeholder(__('My Backup Server'))
                                ->required(),
                            // SFTP fields
                            Grid::make(2)
                                ->schema([
                                    TextInput::make('host')
                                        ->label(__('Host'))
                                        ->required(),
                                    TextInput::make('port')
                                        ->label(__('Port'))
                                        ->numeric()
                                        ->default(22)
                                        ->minValue(1)
                                        ->maxValue(65535),
                                ])
                                ->visible(fn ($get) => $get('dest_type') === 'sftp'),
                            TextInput::make('username')
                                ->label(__('Username'))
                                ->visible(fn ($get) => $get('dest_type') === 'sftp')
                                ->required(fn ($get) => $get('dest_type') === 'sftp'),
                            TextInput::make('password')
                                ->label(__('Password'))
                                ->password()
                                ->visible(fn ($get) => $get('dest_type') === 'sftp'),
                            Textarea::make('private_key')
                                ->label(__('SSH Private Key'))
                                ->rows(3)
                                ->visible(fn ($get) => $get('dest_type') === 'sftp')
                                ->helperText(__('Optional, alternative to password')),
                            TextInput::make('path')
                                ->label(__('Remote Path'))
                                ->default('/backups')
                                ->visible(fn ($get) => $get('dest_type') === 'sftp'),
                            // S3-compatible fields
                            TextInput::make('endpoint')
                                ->label(__('Endpoint'))
                                ->visible(fn ($get) => in_array($get('dest_type'), ['s3', 'b2', 'wasabi', 'minio']))
                                ->required(fn ($get) => in_array($get('dest_type'), ['s3', 'b2', 'wasabi', 'minio'])),
                            TextInput::make('bucket')
                                ->label(__('Bucket'))
                                ->visible(fn ($get) => in_array($get('dest_type'), ['s3', 'b2', 'wasabi', 'minio']))
                                ->required(fn ($get) => in_array($get('dest_type'), ['s3', 'b2', 'wasabi', 'minio'])),
                            TextInput::make('access_key')
                                ->label(__('Access Key'))
                                ->visible(fn ($get) => in_array($get('dest_type'), ['s3', 'b2', 'wasabi', 'minio']))
                                ->required(fn ($get) => in_array($get('dest_type'), ['s3', 'b2', 'wasabi', 'minio'])),
                            TextInput::make('secret_key')
                                ->label(__('Secret Key'))
                                ->password()
                                ->visible(fn ($get) => in_array($get('dest_type'), ['s3', 'b2', 'wasabi', 'minio']))
                                ->required(fn ($get) => in_array($get('dest_type'), ['s3', 'b2', 'wasabi', 'minio'])),
                            // REST server
                            TextInput::make('rest_url')
                                ->label(__('REST Server URL'))
                                ->placeholder('https://backup.example.com:8000')
                                ->visible(fn ($get) => $get('dest_type') === 'rest')
                                ->required(fn ($get) => $get('dest_type') === 'rest'),
                            TextInput::make('rest_username')
                                ->label(__('Username'))
                                ->visible(fn ($get) => $get('dest_type') === 'rest'),
                            TextInput::make('rest_password')
                                ->label(__('Password'))
                                ->password()
                                ->visible(fn ($get) => $get('dest_type') === 'rest'),
                        ])
                        ->afterValidation(function ($get) {
                            $type = $get('dest_type');
                            $data = [
                                'host' => $get('host'),
                                'port' => $get('port'),
                                'username' => $get('username'),
                                'password' => $get('password'),
                                'private_key' => $get('private_key'),
                                'path' => $get('path'),
                                'endpoint' => $get('endpoint'),
                                'bucket' => $get('bucket'),
                                'access_key' => $get('access_key'),
                                'secret_key' => $get('secret_key'),
                                'rest_url' => $get('rest_url'),
                                'rest_username' => $get('rest_username'),
                                'rest_password' => $get('rest_password'),
                            ];

                            $config = $this->buildConfig($type, $data);

                            // Delete previous test destination if retrying
                            if ($this->wizardDestinationId) {
                                BackupDestination::find($this->wizardDestinationId)?->delete();
                                $this->wizardDestinationId = null;
                            }

                            $dest = BackupDestination::create([
                                'name' => $get('dest_name'),
                                'type' => $type,
                                'config' => $config,
                                'is_server_backup' => true,
                                'is_active' => true,
                            ]);

                            try {
                                app(BackupOrchestrator::class)->testDestination($dest);
                            } catch (Exception $e) {
                                $dest->delete();

                                Notification::make()
                                    ->title(__('Connection failed'))
                                    ->body(SafeError::message($e))
                                    ->danger()
                                    ->send();

                                $this->halt();

                                return;
                            }

                            if ($dest->fresh()->test_status !== 'success') {
                                $errorMessage = $dest->fresh()->test_message ?? __('Could not connect to the remote destination.');
                                $dest->delete();

                                Notification::make()
                                    ->title(__('Connection test failed'))
                                    ->body($errorMessage)
                                    ->danger()
                                    ->send();

                                $this->halt();

                                return;
                            }

                            $this->wizardDestinationId = $dest->id;

                            Notification::make()
                                ->title(__('Connection successful'))
                                ->success()
                                ->send();
                        }),

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
                        ]),
                ])
                    ->extraAlpineAttributes([
                        'x-init' => "
                        const swapNextLabel = () => {
                            const lbl = getStepIndex(step) === 1 ? '".__('Validate')."' : '".__('Next')."';
                            const footer = \$el.querySelector('.fi-sc-wizard-footer');
                            if (!footer) return;
                            const btns = footer.querySelectorAll(':scope > div > button, :scope > div button');
                            btns.forEach(b => {
                                const t = b.querySelector('span') || b;
                                const txt = t.textContent.trim();
                                if (['Next', 'Validate', '".__('Next')."', '".__('Validate')."'].includes(txt)) t.textContent = lbl;
                            });
                        };
                        \$watch('step', () => \$nextTick(swapNextLabel));
                        \$nextTick(swapNextLabel);
                    ",
                    ])
                    ->submitAction(new HtmlString(
                        '<button type="submit" wire:click="callMountedAction" class="fi-btn fi-btn-size-md fi-color-primary fi-btn-color-primary relative grid-flow-col items-center justify-center gap-1.5 rounded-lg px-3 py-2 text-sm font-semibold outline-none transition duration-75 fi-ac-action fi-ac-btn-action shadow-sm bg-primary-600 text-white hover:bg-primary-500 dark:bg-primary-500 dark:hover:bg-primary-400">'.__('Save & Create Schedule').'</button>'
                    )),
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

                // 2. Use destination created and tested in the Destination step
                $destinationId = $this->wizardDestinationId;

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

                // 4. Mark wizard as completed
                DnsSetting::set('backup_wizard_completed', '1');
                DnsSetting::clearCache();

                Notification::make()
                    ->title(__('Backup setup complete'))
                    ->body(__('Your encryption password has been saved and a backup schedule has been created.'))
                    ->success()
                    ->send();
            });
    }
}
