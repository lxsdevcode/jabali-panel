<?php

declare(strict_types=1);

namespace App\Filament\Admin\Pages;

use App\FileBrowser\Support\Formatter;
use App\Models\Backup;
use App\Services\Agent\AgentClient;
use App\Services\Agent\InteractsWithAgent;
use App\Services\Backup\BackupOrchestrator;
use App\Services\FileBrowser\BackupSnapshotAdapter;
use App\Support\SafeError;
use BackedEnum;
use Exception;
use Filament\Actions\Action;
use Filament\Actions\Concerns\InteractsWithActions;
use Filament\Actions\Contracts\HasActions;
use Filament\Forms\Components\Checkbox;
use Filament\Forms\Components\Radio;
use Filament\Forms\Components\Select;
use Filament\Forms\Concerns\InteractsWithForms;
use Filament\Forms\Contracts\HasForms;
use Filament\Notifications\Notification;
use Filament\Pages\Page;
use Filament\Schemas\Schema;
use Filament\Tables\Columns\TextColumn;
use Filament\Tables\Concerns\InteractsWithTable;
use Filament\Tables\Contracts\HasTable;
use Filament\Tables\Table;
use Illuminate\Contracts\Support\Htmlable;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Support\HtmlString;

class RestoreBackup extends Page implements HasActions, HasForms, HasTable
{
    use InteractsWithActions;
    use InteractsWithAgent;
    use InteractsWithForms;
    use InteractsWithTable;

    protected static string|BackedEnum|null $navigationIcon = 'heroicon-o-arrow-path';

    protected static bool $shouldRegisterNavigation = false;

    protected string $view = 'filament.admin.pages.restore-backup';

    public ?int $backupId = null;

    public ?string $restoreUsername = null;

    public string $currentPath = '';

    public array $selectedFiles = [];

    public bool $restoreFiles = false;

    public string $filesRestoreMode = 'domains';

    public bool $restoreDatabases = false;

    public bool $restoreMysqlUsers = false;

    public bool $restoreMailboxes = false;

    public bool $restoreSsl = false;

    public bool $restoreDns = false;

    protected static ?string $slug = 'backups/restore/{backupId}';

    public function getTitle(): string|Htmlable
    {
        $backup = $this->getBackup();

        return $backup ? __('Restore: :name', ['name' => $backup->name]) : __('Restore Backup');
    }

    public function mount(int $backupId): void
    {
        $this->backupId = $backupId;

        $backup = $this->getBackup();
        if (! $backup) {
            Notification::make()->title(__('Backup not found'))->danger()->send();
            $this->redirectRoute('filament.admin.pages.backups', ['tab' => 'backups']);

            return;
        }

        // Pre-select first user if only one
        $users = $this->getUsers();
        if (count($users) === 1) {
            $this->restoreUsername = array_key_first($users);
        }
    }

    protected function getBackup(): ?Backup
    {
        $backup = Backup::with('destination')->find($this->backupId);

        // Only server backups (admin-created) are accessible from admin panel
        if ($backup && $backup->type !== 'server' && $backup->user_id !== null) {
            abort(403);
        }

        return $backup;
    }

    public function isIncremental(): bool
    {
        $backup = $this->getBackup();
        if (! $backup) {
            return false;
        }

        return ($backup->metadata['backup_type'] ?? 'full') === 'incremental';
    }

    public function restoreOptionsForm(Schema $schema): Schema
    {
        return $schema
            ->schema([
                Select::make('restoreUsername')
                    ->label(__('User to Restore'))
                    ->options(fn (): array => $this->getUsers())
                    ->placeholder(__('Select a user...'))
                    ->searchable()
                    ->live()
                    ->columnSpanFull(),

                Checkbox::make('restoreFiles')
                    ->label(__('Website Files'))
                    ->live()
                    ->visible(fn (): bool => filled($this->restoreUsername) && $this->isIncremental()),

                Radio::make('filesRestoreMode')
                    ->label(__('Files Restore Mode'))
                    ->options([
                        'domains' => __('Full Domains'),
                        'files' => __('Specific Files / Folders'),
                    ])
                    ->inline()
                    ->visible(fn (): bool => $this->restoreFiles && $this->restoreUsername !== '__all__' && $this->isIncremental())
                    ->live(),

                Checkbox::make('restoreDatabases')
                    ->label(__('Databases'))
                    ->visible(fn (): bool => filled($this->restoreUsername) && $this->isIncremental()),

                Checkbox::make('restoreMysqlUsers')
                    ->label(__('MySQL Users'))
                    ->visible(fn (): bool => filled($this->restoreUsername) && $this->isIncremental()),

                Checkbox::make('restoreMailboxes')
                    ->label(__('Mailboxes'))
                    ->visible(fn (): bool => filled($this->restoreUsername) && $this->isIncremental()),

                Checkbox::make('restoreSsl')
                    ->label(__('SSL Certificates'))
                    ->visible(fn (): bool => filled($this->restoreUsername) && $this->isIncremental()),

                Checkbox::make('restoreDns')
                    ->label(__('DNS Zones'))
                    ->visible(fn (): bool => filled($this->restoreUsername) && $this->isIncremental()),
            ]);
    }

    protected function getForms(): array
    {
        return [
            'restoreOptionsForm',
        ];
    }

    protected function getUsers(): array
    {
        $backup = $this->getBackup();
        if (! $backup) {
            return [];
        }

        $orchestrator = app(BackupOrchestrator::class);
        $manifest = $orchestrator->getBackupManifest($backup);
        $users = $manifest['users'] ?? $backup->users ?? [];
        if (empty($users)) {
            $users = [$manifest['username'] ?? ''];
        }

        $users = array_filter($users);
        $options = ['__all__' => __('All users')];
        foreach ($users as $u) {
            $options[$u] = $u;
        }

        return $options;
    }

    protected function getContentsForUser(?string $username): array
    {
        if (empty($username) || $username === '__all__') {
            return [];
        }

        $backup = $this->getBackup();
        if (! $backup) {
            return [];
        }

        $isRemote = ! $backup->local_path || ! file_exists($backup->local_path);
        if ($isRemote && $backup->destination) {
            try {
                $destConfig = array_merge(
                    $backup->destination->config ?? [],
                    ['type' => $backup->destination->type]
                );

                return $this->agent()->backupListContents(
                    $backup->remote_path ?? '',
                    $username,
                    $destConfig
                );
            } catch (Exception) {
            }
        }

        $orchestrator = app(BackupOrchestrator::class);

        return $orchestrator->getBackupManifest($backup, $username);
    }

    // ─── File Browser Table ─────────────────────────────────────────

    public function navigateTo(string $path): void
    {
        $this->currentPath = $path;
        $this->resetTable();
    }

    public function toggleFile(string $path): void
    {
        // Reject path traversal attempts
        if (str_contains($path, '..') || str_starts_with($path, '/')) {
            return;
        }

        if (in_array($path, $this->selectedFiles)) {
            $this->selectedFiles = array_values(array_diff($this->selectedFiles, [$path]));
        } else {
            $this->selectedFiles[] = $path;
        }
    }

    public function clearSelectedFiles(): void
    {
        $this->selectedFiles = [];
    }

    public function table(Table $table): Table
    {
        return $table
            ->records(function (): array {
                if ($this->filesRestoreMode !== 'files' || empty($this->restoreUsername) || $this->restoreUsername === '__all__') {
                    return [];
                }

                $backup = $this->getBackup();
                if (! $backup || ! $backup->destination) {
                    return [];
                }

                try {
                    $config = array_merge($backup->destination->config ?? [], ['type' => $backup->destination->type]);
                    $adapter = new BackupSnapshotAdapter(
                        app(AgentClient::class),
                        $backup->remote_path ?? $backup->local_path ?? '',
                        $this->restoreUsername,
                        $config,
                    );
                    $result = $adapter->files()->list($this->currentPath);
                    $items = $result['items'] ?? [];
                    $indexed = [];
                    foreach ($items as $i => $item) {
                        $item['id'] = $i + 1;
                        $item['selected'] = in_array($item['path'] ?? '', $this->selectedFiles);
                        $indexed[$item['path'] ?? $i + 1] = $item;
                    }

                    return $indexed;
                } catch (\Throwable $e) {
                    Notification::make()->title(__('Error loading directory'))->body($e->getMessage())->danger()->send();

                    return [];
                }
            })
            ->columns([
                TextColumn::make('name')
                    ->label(__('Name'))
                    ->icon(fn (array $record): string => match (true) {
                        $record['is_parent'] ?? false => 'heroicon-o-arrow-uturn-left',
                        $record['is_dir'] ?? false => 'heroicon-o-folder',
                        default => 'heroicon-o-document',
                    })
                    ->iconColor(fn (array $record): string => match (true) {
                        $record['is_parent'] ?? false => 'gray',
                        $record['is_dir'] ?? false => 'warning',
                        default => 'gray',
                    })
                    ->weight(fn (array $record): string => ($record['is_dir'] ?? false) ? 'medium' : 'normal')
                    ->formatStateUsing(function (string $state, array $record): HtmlString {
                        $name = e($state);
                        if ($record['is_dir'] ?? false) {
                            $path = e($record['path'] ?? '');

                            return new HtmlString(
                                "<button type=\"button\" wire:click=\"navigateTo('{$path}')\" class=\"text-left hover:underline cursor-pointer\">{$name}</button>"
                            );
                        }

                        return new HtmlString($name);
                    })
                    ->html(),
                TextColumn::make('size')
                    ->label(__('Size'))
                    ->formatStateUsing(fn ($state): string => $state ? Formatter::bytes((int) $state) : '-')
                    ->color('gray'),
                TextColumn::make('permissions')
                    ->label(__('Permissions'))
                    ->badge()
                    ->color('gray'),
                TextColumn::make('modified')
                    ->label(__('Modified'))
                    ->formatStateUsing(fn ($state): string => $state ? date('M j, Y H:i', (int) $state) : '-')
                    ->color('gray'),
            ])
            ->recordActions([
                \Filament\Actions\Action::make('select')
                    ->label(fn (array $record): string => in_array($record['path'] ?? '', $this->selectedFiles) ? __('Deselect') : __('Select'))
                    ->icon(fn (array $record): string => in_array($record['path'] ?? '', $this->selectedFiles) ? 'heroicon-o-check-circle' : 'heroicon-o-plus-circle')
                    ->color(fn (array $record): string => in_array($record['path'] ?? '', $this->selectedFiles) ? 'success' : 'gray')
                    ->size('sm')
                    ->visible(fn (array $record): bool => ! ($record['is_parent'] ?? false))
                    ->action(fn (array $record) => $this->toggleFile($record['path'] ?? '')),
            ])
            ->paginated(false)
            ->emptyStateHeading($this->filesRestoreMode === 'files' ? __('Select a user and enable file browsing') : __('Switch to "Specific Files" mode'))
            ->emptyStateIcon('heroicon-o-folder')
            ->striped();
    }

    public function getTableRecordKey(Model|array $record): string
    {
        return $record['path'] ?? (string) ($record['id'] ?? 0);
    }

    public function getBreadcrumbs(): array
    {
        $crumbs = ['' => __('Home')];
        if (empty($this->currentPath)) {
            return $crumbs;
        }

        $parts = explode('/', $this->currentPath);
        $path = '';
        foreach ($parts as $part) {
            $path = $path ? $path.'/'.$part : $part;
            $crumbs[$path] = $part;
        }

        return $crumbs;
    }

    // ─── Restore Action ─────────────────────────────────────────────

    public function startRestore(): void
    {
        // Rate limit: 1 restore per minute per admin
        $key = 'restore-backup:'.auth()->id();
        if (! \Illuminate\Support\Facades\RateLimiter::attempt($key, 1, fn () => true, 60)) {
            Notification::make()->title(__('Please wait before starting another restore'))->warning()->send();

            return;
        }

        $backup = $this->getBackup();
        if (! $backup) {
            Notification::make()->title(__('Backup not found'))->danger()->send();

            return;
        }

        // Validate username against known users from backup manifest
        $validUsers = $this->getUsers();
        if ($this->restoreUsername !== '__all__' && ! array_key_exists($this->restoreUsername, $validUsers)) {
            Notification::make()->title(__('Invalid user selected'))->danger()->send();

            return;
        }

        if (empty($this->restoreUsername)) {
            Notification::make()->title(__('Please select a user'))->warning()->send();

            return;
        }

        // For full backups, restore everything
        if (! $this->isIncremental()) {
            $this->restoreFiles = true;
            $this->restoreDatabases = true;
            $this->restoreMysqlUsers = true;
            $this->restoreMailboxes = true;
            $this->restoreSsl = true;
            $this->restoreDns = true;
        }

        if (! $this->restoreFiles && ! $this->restoreDatabases && ! $this->restoreMailboxes && ! $this->restoreSsl && ! $this->restoreDns) {
            Notification::make()->title(__('Please select at least one item to restore'))->warning()->send();

            return;
        }

        $data = [
            'restore_username' => $this->restoreUsername,
            'restore_files' => $this->restoreFiles,
            'restore_databases' => $this->restoreDatabases,
            'restore_mysql_users' => $this->restoreMysqlUsers,
            'restore_mailboxes' => $this->restoreMailboxes,
            'restore_ssl' => $this->restoreSsl,
            'restore_dns' => $this->restoreDns,
            'files_restore_mode' => $this->isIncremental() ? $this->filesRestoreMode : 'domains',
            'selected_files' => $this->isIncremental()
                ? array_filter($this->selectedFiles, fn (string $p): bool => ! str_contains($p, '..') && ! str_starts_with($p, '/'))
                : [],
        ];

        $this->executeRestore($backup, $data);
    }

    protected function executeRestore(Backup $backup, array $data): void
    {
        $username = $data['restore_username'];
        $isRemote = ! $backup->local_path || ! file_exists($backup->local_path);

        if ($username === '__all__') {
            $users = $this->getUsers();
            unset($users['__all__']);

            foreach ($users as $user => $label) {
                $this->restoreForUser($backup, $user, $data, $isRemote);
            }
        } else {
            $this->restoreForUser($backup, $username, $data, $isRemote);
        }
    }

    protected function restoreForUser(Backup $backup, string $username, array $data, bool $isRemote): void
    {
        // Reject path traversal in username
        if (str_contains($username, '..') || str_contains($username, '/') || empty($username)) {
            Notification::make()->title(__('Invalid username'))->danger()->send();

            return;
        }

        $tempDownloadPath = null;
        $backupPath = $backup->local_path;

        try {
            if ($isRemote && $backup->destination) {
                $tempDownloadPath = '/tmp/jabali_restore_download_'.uniqid();
                $config = array_merge($backup->destination->config ?? [], ['type' => $backup->destination->type]);

                Notification::make()->title(__('Downloading backup'))->body(__('Downloading from remote destination...'))->info()->send();

                $downloadResult = $this->agent()->send('backup.download_remote', [
                    'remote_path' => $backup->remote_path.'/'.$username,
                    'local_path' => $tempDownloadPath,
                    'destination' => $config,
                ]);

                if (! ($downloadResult['success'] ?? false)) {
                    throw new Exception($downloadResult['error'] ?? __('Download failed'));
                }

                $backupPath = $tempDownloadPath;
            }

            $options = [
                'restore_files' => $data['restore_files'],
                'restore_databases' => $data['restore_databases'],
                'restore_mailboxes' => $data['restore_mailboxes'],
                'restore_dns' => $data['restore_dns'],
                'restore_ssl' => $data['restore_ssl'],
                'restore_mysql_users' => $data['restore_mysql_users'],
            ];

            // Pass selected files for file-level restore
            if ($data['files_restore_mode'] === 'files' && ! empty($data['selected_files'])) {
                $options['selected_files'] = $data['selected_files'];
            }

            $result = $this->agent()->send('backup.restore', [
                'username' => $username,
                'backup_path' => $backupPath,
                ...$options,
            ]);

            // Cleanup
            if ($tempDownloadPath && is_dir($tempDownloadPath)) {
                try {
                    \Illuminate\Support\Facades\File::deleteDirectory($tempDownloadPath);
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

                Notification::make()
                    ->title(__('Restore completed for :user', ['user' => $username]))
                    ->body(implode(', ', $summary))
                    ->success()
                    ->send();
            } else {
                throw new Exception($result['error'] ?? __('Restore failed'));
            }
        } catch (Exception $e) {
            if ($tempDownloadPath && is_dir($tempDownloadPath)) {
                try {
                    \Illuminate\Support\Facades\File::deleteDirectory($tempDownloadPath);
                } catch (\Throwable) {
                    try {
                        $this->agent()->backupDeleteServer($tempDownloadPath);
                    } catch (\Throwable) {
                    }
                }
            }

            Notification::make()
                ->title(__('Restore failed for :user', ['user' => $username]))
                ->body(SafeError::message($e))
                ->danger()
                ->send();
        }
    }
}
