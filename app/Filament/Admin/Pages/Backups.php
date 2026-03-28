<?php

declare(strict_types=1);

namespace App\Filament\Admin\Pages;

use App\Jobs\RunServerBackup;
use App\Models\Backup;
use App\Models\BackupDestination;
use App\Models\User;
use App\Services\Backup\BackupOrchestrator;
use App\Support\SafeError;
use BackedEnum;
use Exception;
use Filament\Actions\Action;
use Filament\Actions\Concerns\InteractsWithActions;
use Filament\Actions\Contracts\HasActions;
use Filament\Forms\Components\CheckboxList;
use Filament\Forms\Components\Select;
use Filament\Forms\Components\Textarea;
use Filament\Forms\Components\TextInput;
use Filament\Forms\Components\Toggle;
use Filament\Forms\Concerns\InteractsWithForms;
use Filament\Forms\Contracts\HasForms;
use Filament\Notifications\Notification;
use Filament\Pages\Page;
use Filament\Schemas\Components\Grid;
use Filament\Tables\Columns\TextColumn;
use Filament\Tables\Concerns\InteractsWithTable;
use Filament\Tables\Contracts\HasTable;
use Filament\Tables\Table;
use Illuminate\Contracts\Support\Htmlable;
use Livewire\Attributes\Url;

class Backups extends Page implements HasActions, HasForms, HasTable
{
    use InteractsWithActions;
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

    // ── Header Actions ──────────────────────────────────────────────────

    protected function getHeaderActions(): array
    {
        return [
            $this->createServerBackupAction(),
            $this->addDestinationAction(),
        ];
    }

    // ── Backups Table ─────────────────────────────────────────────────

    public function table(Table $table): Table
    {
        return $table
            ->query(Backup::query()->latest())
            ->columns([
                TextColumn::make('name')
                    ->label(__('Name'))
                    ->searchable()
                    ->limit(40),
                TextColumn::make('status')
                    ->label(__('Status'))
                    ->badge()
                    ->color(fn (string $state) => match ($state) {
                        'completed' => 'success',
                        'running' => 'warning',
                        'pending' => 'gray',
                        'failed' => 'danger',
                        default => 'gray',
                    }),
                TextColumn::make('size_bytes')
                    ->label(__('Size'))
                    ->formatStateUsing(fn ($state) => $state > 0 ? \App\Support\Formatter::bytes($state) : '-'),
                TextColumn::make('destination.name')
                    ->label(__('Destination'))
                    ->placeholder(__('Local')),
                TextColumn::make('created_at')
                    ->label(__('Created'))
                    ->dateTime('M j, Y H:i')
                    ->sortable(),
                TextColumn::make('snapshot_id')
                    ->label(__('Snapshot'))
                    ->limit(8)
                    ->placeholder('-')
                    ->toggleable(isToggledHiddenByDefault: true),
            ])
            ->actions([
                Action::make('delete')
                    ->label(__('Delete'))
                    ->icon('heroicon-o-trash')
                    ->color('danger')
                    ->requiresConfirmation()
                    ->modalDescription(__('This will permanently delete the backup snapshot. This cannot be undone.'))
                    ->action(function (Backup $record): void {
                        try {
                            app(BackupOrchestrator::class)->deleteBackup($record);
                            Notification::make()->title(__('Backup deleted'))->success()->send();
                        } catch (Exception $e) {
                            Notification::make()
                                ->title(__('Delete failed'))
                                ->body(SafeError::message($e))
                                ->danger()
                                ->send();
                        }
                    }),
            ])
            ->poll('15s')
            ->emptyStateHeading(__('No backups yet'))
            ->emptyStateDescription(__('Click "Create Server Backup" to create your first backup'))
            ->emptyStateIcon('heroicon-o-cloud-arrow-up')
            ->defaultSort('created_at', 'desc');
    }

    // ── Backup Actions ────────────────────────────────────────────────

    private function createServerBackupAction(): Action
    {
        $destinations = BackupDestination::where('is_server_backup', true)
            ->where('is_active', true)
            ->pluck('name', 'id')
            ->toArray();

        $users = User::where('is_active', true)
            ->pluck('username', 'id')
            ->toArray();

        return Action::make('createServerBackup')
            ->label(__('Create Server Backup'))
            ->icon('heroicon-o-cloud-arrow-up')
            ->color('danger')
            ->modalHeading(__('Create Server Backup'))
            ->form([
                Select::make('destination_id')
                    ->label(__('Destination'))
                    ->options(array_merge(['' => __('Local (default)')], $destinations))
                    ->placeholder(__('Select destination')),
                CheckboxList::make('selected_users')
                    ->label(__('Users'))
                    ->options($users)
                    ->columns(3)
                    ->helperText(__('Leave empty to backup all users')),
                Grid::make(3)->schema([
                    Toggle::make('include_files')->label(__('Files'))->default(true),
                    Toggle::make('include_databases')->label(__('Databases'))->default(true),
                    Toggle::make('include_mailboxes')->label(__('Mailboxes'))->default(true),
                ]),
                Grid::make(2)->schema([
                    Toggle::make('include_dns')->label(__('DNS Zones'))->default(true),
                    Toggle::make('include_ssl')->label(__('SSL Certificates'))->default(true),
                ]),
            ])
            ->action(function (array $data): void {
                $destinationId = ! empty($data['destination_id']) ? (int) $data['destination_id'] : null;
                $selectedUsers = ! empty($data['selected_users'])
                    ? User::whereIn('id', $data['selected_users'])->pluck('username')->toArray()
                    : null;

                $name = 'Server Backup '.now()->format('Y-m-d H:i');

                $backup = Backup::create([
                    'name' => $name,
                    'filename' => 'restic-snapshot',
                    'type' => 'server',
                    'destination_id' => $destinationId,
                    'users' => $selectedUsers,
                    'include_files' => $data['include_files'] ?? true,
                    'include_databases' => $data['include_databases'] ?? true,
                    'include_mailboxes' => $data['include_mailboxes'] ?? true,
                    'include_dns' => $data['include_dns'] ?? true,
                    'include_ssl' => $data['include_ssl'] ?? true,
                    'status' => 'pending',
                ]);

                RunServerBackup::dispatch($backup->id);

                Notification::make()
                    ->title(__('Backup started'))
                    ->body(__('Server backup is running in the background.'))
                    ->success()
                    ->send();
            });
    }

    // ── Destination Actions ─────────────────────────────────────────────

    private function addDestinationAction(): Action
    {
        return Action::make('addDestination')
            ->label(__('Add Destination'))
            ->icon('heroicon-o-plus')
            ->form([
                Select::make('type')
                    ->label(__('Type'))
                    ->options([
                        'sftp' => __('SFTP Server'),
                        's3' => __('S3-Compatible Storage'),
                        'local' => __('Local Path'),
                    ])
                    ->required()
                    ->live(),
                TextInput::make('name')
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
                            ->default(22),
                    ])
                    ->visible(fn ($get) => $get('type') === 'sftp'),
                TextInput::make('username')
                    ->label(__('Username'))
                    ->visible(fn ($get) => $get('type') === 'sftp')
                    ->required(fn ($get) => $get('type') === 'sftp'),
                TextInput::make('password')
                    ->label(__('Password'))
                    ->password()
                    ->visible(fn ($get) => $get('type') === 'sftp'),
                Textarea::make('private_key')
                    ->label(__('SSH Private Key'))
                    ->rows(3)
                    ->visible(fn ($get) => $get('type') === 'sftp')
                    ->helperText(__('Paste private key here (optional, alternative to password)')),
                TextInput::make('path')
                    ->label(__('Remote Path'))
                    ->default('/backups')
                    ->visible(fn ($get) => in_array($get('type'), ['sftp', 'local'])),
                // S3 fields
                TextInput::make('endpoint')
                    ->label(__('S3 Endpoint'))
                    ->placeholder('https://s3.amazonaws.com')
                    ->visible(fn ($get) => $get('type') === 's3')
                    ->required(fn ($get) => $get('type') === 's3'),
                TextInput::make('bucket')
                    ->label(__('Bucket'))
                    ->visible(fn ($get) => $get('type') === 's3')
                    ->required(fn ($get) => $get('type') === 's3'),
                TextInput::make('access_key')
                    ->label(__('Access Key'))
                    ->visible(fn ($get) => $get('type') === 's3')
                    ->required(fn ($get) => $get('type') === 's3'),
                TextInput::make('secret_key')
                    ->label(__('Secret Key'))
                    ->password()
                    ->visible(fn ($get) => $get('type') === 's3')
                    ->required(fn ($get) => $get('type') === 's3'),
            ])
            ->action(function (array $data): void {
                $type = $data['type'];
                $config = $this->buildConfig($type, $data);

                // Test connection first
                try {
                    $orchestrator = app(BackupOrchestrator::class);
                    $dest = BackupDestination::create([
                        'name' => $data['name'],
                        'type' => $type,
                        'config' => $config,
                        'is_server_backup' => true,
                        'is_active' => true,
                    ]);

                    $orchestrator->testDestination($dest);

                    if ($dest->test_status === 'failed') {
                        Notification::make()
                            ->title(__('Destination added but connection failed'))
                            ->body($dest->test_message ?? __('Check credentials'))
                            ->warning()
                            ->send();
                    } else {
                        Notification::make()
                            ->title(__('Destination added'))
                            ->success()
                            ->send();
                    }
                } catch (Exception $e) {
                    Notification::make()
                        ->title(__('Failed to add destination'))
                        ->body(SafeError::message($e))
                        ->danger()
                        ->send();
                }
            });
    }

    public function testDestination(int $id): void
    {
        $destination = BackupDestination::find($id);
        if (! $destination) {
            return;
        }

        try {
            $orchestrator = app(BackupOrchestrator::class);
            $orchestrator->testDestination($destination);

            if ($destination->fresh()->test_status === 'success') {
                Notification::make()->title(__('Connection successful'))->success()->send();
            } else {
                Notification::make()
                    ->title(__('Connection failed'))
                    ->body($destination->fresh()->test_message ?? __('Unknown error'))
                    ->danger()
                    ->send();
            }
        } catch (Exception $e) {
            Notification::make()
                ->title(__('Connection failed'))
                ->body(SafeError::message($e))
                ->danger()
                ->send();
        }
    }

    private function updateDestination(BackupDestination $dest, array $data): void
    {
        $config = $this->buildConfig($dest->type, $data);
        $dest->update([
            'name' => $data['name'] ?? $dest->name,
            'config' => $config,
        ]);

        Notification::make()->title(__('Destination updated'))->success()->send();
    }

    // ── Helpers ─────────────────────────────────────────────────────────

    private function buildConfig(string $type, array $data): array
    {
        return match ($type) {
            'sftp' => [
                'type' => 'sftp',
                'host' => $data['host'] ?? '',
                'port' => (int) ($data['port'] ?? 22),
                'username' => $data['username'] ?? '',
                'password' => $data['password'] ?? '',
                'private_key' => $data['private_key'] ?? '',
                'path' => $data['path'] ?? '/backups',
            ],
            's3' => [
                'type' => 's3',
                'endpoint' => $data['endpoint'] ?? '',
                'bucket' => $data['bucket'] ?? '',
                'access_key' => $data['access_key'] ?? '',
                'secret_key' => $data['secret_key'] ?? '',
            ],
            default => [
                'type' => 'local',
                'path' => $data['path'] ?? '/var/backups/jabali/restic',
            ],
        };
    }

    /**
     * @return array<int, \Filament\Forms\Components\Component>
     */
    private function destinationFormFields(string $type, array $config = []): array
    {
        $fields = [
            TextInput::make('name')->label(__('Name'))->required(),
        ];

        if ($type === 'sftp') {
            $fields[] = Grid::make(2)->schema([
                TextInput::make('host')->label(__('Host'))->default($config['host'] ?? '')->required(),
                TextInput::make('port')->label(__('Port'))->numeric()->default($config['port'] ?? 22),
            ]);
            $fields[] = TextInput::make('username')->label(__('Username'))->default($config['username'] ?? '')->required();
            $fields[] = TextInput::make('password')->label(__('Password'))->password()->default($config['password'] ?? '');
            $fields[] = TextInput::make('path')->label(__('Remote Path'))->default($config['path'] ?? '/backups');
        } elseif ($type === 's3') {
            $fields[] = TextInput::make('endpoint')->label(__('Endpoint'))->default($config['endpoint'] ?? '')->required();
            $fields[] = TextInput::make('bucket')->label(__('Bucket'))->default($config['bucket'] ?? '')->required();
            $fields[] = TextInput::make('access_key')->label(__('Access Key'))->default($config['access_key'] ?? '')->required();
            $fields[] = TextInput::make('secret_key')->label(__('Secret Key'))->password()->default($config['secret_key'] ?? '')->required();
        } else {
            $fields[] = TextInput::make('path')->label(__('Path'))->default($config['path'] ?? '/var/backups/jabali/restic');
        }

        return $fields;
    }
}
