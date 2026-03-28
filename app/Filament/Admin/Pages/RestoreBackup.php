<?php

declare(strict_types=1);

namespace App\Filament\Admin\Pages;

use App\Models\Backup;
use App\Models\BackupDestination;
use App\Models\User;
use App\Services\Agent\AgentClient;
use App\Services\Backup\BackupOrchestrator;
use App\Services\FileBrowser\BackupSnapshotAdapter;
use App\Support\SafeError;
use BackedEnum;
use Exception;
use Filament\Actions\Action;
use Filament\Actions\Concerns\InteractsWithActions;
use Filament\Actions\Contracts\HasActions;
use Filament\Forms\Components\Placeholder;
use Filament\Forms\Components\Select;
use Filament\Forms\Concerns\InteractsWithForms;
use Filament\Forms\Contracts\HasForms;
use Filament\Notifications\Notification;
use Filament\Pages\Page;
use Filament\Schemas\Components\View;
use Filament\Schemas\Components\Wizard;
use Filament\Schemas\Components\Wizard\Step;
use Filament\Schemas\Schema;
use Filament\Support\Icons\Heroicon;
use Illuminate\Contracts\Support\Htmlable;
use Illuminate\Support\HtmlString;

class RestoreBackup extends Page implements HasActions, HasForms
{
    use InteractsWithActions;
    use InteractsWithForms;

    protected static ?string $slug = 'backups/restore/{backupId}';

    protected static bool $shouldRegisterNavigation = false;

    protected static string|BackedEnum|null $navigationIcon = Heroicon::OutlinedArrowPath;

    protected string $view = 'filament.admin.pages.restore-backup';

    // ── Wizard State ────────────────────────────────────────────────────

    public ?int $backupId = null;

    public ?string $selectedUser = null;

    public array $contents = [];

    public array $selectedDatabases = [];

    public array $selectedMailboxes = [];

    public string $activeSection = 'files';

    public string $currentPath = '';

    public array $selectedPaths = [];

    public array $directoryItems = [];

    public string $conflictMode = 'overwrite';

    public bool $restoreInProgress = false;

    // ── Mount ───────────────────────────────────────────────────────────

    public function mount(?int $backupId = null): void
    {
        $this->backupId = $backupId ?? (int) request()->route('backupId');
        $backup = $this->getBackup();

        if (! $backup || ! $backup->snapshot_id) {
            $this->redirect('/jabali-admin/backups');

            return;
        }
    }

    public function getTitle(): string|Htmlable
    {
        return __('Restore Wizard');
    }

    // ── Header Actions ──────────────────────────────────────────────────

    protected function getHeaderActions(): array
    {
        return [
            Action::make('back')
                ->label(__('Back to Backups'))
                ->icon('heroicon-o-arrow-left')
                ->color('gray')
                ->url('/jabali-admin/backups'),
        ];
    }

    // ── Schema ─────────────────────────────────────────────────────────

    protected function getForms(): array
    {
        return ['restoreForm'];
    }

    public function restoreForm(Schema $schema): Schema
    {
        return $schema->schema([
            Wizard::make([
                Step::make(__('Account'))
                    ->icon(Heroicon::OutlinedUser)
                    ->schema([
                        Select::make('selectedUser')
                            ->label(__('Restore for user'))
                            ->options(fn () => User::where('is_active', true)->pluck('username', 'username')->toArray())
                            ->required()
                            ->live()
                            ->searchable(),
                        Placeholder::make('backup_info')
                            ->label(__('Backup'))
                            ->content(function (): HtmlString {
                                $backup = $this->getBackup();
                                if (! $backup) {
                                    return new HtmlString('-');
                                }

                                return new HtmlString(
                                    e($backup->name)
                                    .' &middot; '
                                    .e($backup->created_at?->format('M j, Y H:i'))
                                    .($backup->size_bytes > 0 ? ' &middot; '.e(\App\Support\Formatter::bytes($backup->size_bytes)) : '')
                                );
                            }),
                    ])
                    ->afterValidation(function (): void {
                        $this->selectedPaths = [];
                        $this->selectedDatabases = [];
                        $this->selectedMailboxes = [];
                        $this->loadContents();
                    }),

                Step::make(__('Contents'))
                    ->icon(Heroicon::OutlinedRectangleGroup)
                    ->schema([
                        View::make('filament.admin.pages.restore-backup-contents'),
                    ]),

                Step::make(__('Select Items'))
                    ->icon(Heroicon::OutlinedListBullet)
                    ->schema([
                        View::make('filament.admin.pages.restore-backup-browser'),
                    ])
                    ->beforeValidation(function (): void {
                        $this->currentPath = $this->selectedUser ?? '';
                        $this->refreshDirectory();
                    }),

                Step::make(__('Confirm'))
                    ->icon(Heroicon::OutlinedCheckCircle)
                    ->schema([
                        Placeholder::make('restore_user')
                            ->label(__('User'))
                            ->content(fn () => $this->selectedUser ?? '-'),
                        Placeholder::make('restore_backup')
                            ->label(__('Backup'))
                            ->content(fn () => $this->getBackup()?->name ?? '-'),
                        View::make('filament.admin.pages.restore-backup-confirm'),
                        Select::make('conflictMode')
                            ->label(__('Conflict resolution'))
                            ->options([
                                'overwrite' => __('Overwrite existing files'),
                                'skip' => __('Skip existing files'),
                            ])
                            ->default('overwrite'),
                    ]),
            ])
                ->submitAction(
                    Action::make('restore')
                        ->label(__('Restore Now'))
                        ->color('danger')
                        ->icon('heroicon-o-arrow-path')
                        ->requiresConfirmation()
                        ->modalDescription(__('Are you sure you want to restore these items?'))
                        ->action(fn () => $this->executeRestore()),
                ),
        ]);
    }

    // ── Step 2: Load Contents ───────────────────────────────────────────

    public function loadContents(): void
    {
        $backup = $this->getBackup();
        if (! $backup) {
            return;
        }

        try {
            $repo = $backup->destination
                ? $backup->destination->getResticRepoUrl()
                : BackupDestination::defaultRepo();
            $destConfig = $backup->destination
                ? array_merge($backup->destination->config ?? [], ['type' => $backup->destination->type])
                : [];

            $agent = app(AgentClient::class);
            $result = $agent->send('backup.list_contents', [
                'snapshot_id' => $backup->snapshot_id,
                'destination' => $destConfig,
                'repo' => $repo,
            ]);

            $files = $result['files'] ?? [];
            $domains = [];
            $databases = [];
            $mailboxes = [];
            $hasDbUsers = false;

            foreach ($files as $file) {
                $parts = explode('/', $file);

                if (str_contains($file, "home/{$this->selectedUser}/domains/") && count($parts) >= 5) {
                    $domIdx = array_search('domains', $parts);
                    if ($domIdx !== false && isset($parts[$domIdx + 1])) {
                        $domains[$parts[$domIdx + 1]] = true;
                    }
                } elseif (str_contains($file, "home/{$this->selectedUser}/.jabali-backup/databases/") && str_ends_with($file, '.sql.gz')) {
                    $databases[] = basename($file, '.sql.gz');
                } elseif (str_contains($file, "home/{$this->selectedUser}/.jabali-backup/databases/users.sql")) {
                    $hasDbUsers = true;
                } elseif (str_starts_with($file, 'var/mail/vhosts/') && count($parts) >= 5) {
                    $mailboxes["{$parts[4]}@{$parts[3]}"] = true;
                }
            }

            $this->contents = [
                'domains' => array_keys($domains),
                'databases' => array_values(array_unique($databases)),
                'mailboxes' => array_keys($mailboxes),
                'has_db_users' => $hasDbUsers,
                'total_files' => count($files),
            ];

            $this->selectedDatabases = $this->contents['databases'];
            $this->selectedMailboxes = $this->contents['mailboxes'];
        } catch (Exception $e) {
            $this->contents = [];
            Notification::make()->title(__('Failed to load contents'))->body(SafeError::message($e))->danger()->send();
        }
    }

    // ── Step 3: File Browser ────────────────────────────────────────────

    public function selectSectionAndRefresh(string $section): void
    {
        $this->activeSection = $section;
        if ($section === 'files') {
            $this->currentPath = $this->selectedUser ?? '';
            $this->refreshDirectory();
        }
    }

    public function navigateTo(string $path): void
    {
        if (str_contains($path, '..')) {
            return;
        }

        $this->currentPath = $path;
        $this->refreshDirectory();
    }

    public function refreshDirectory(): void
    {
        $backup = $this->getBackup();
        if (! $backup || ! $this->selectedUser) {
            $this->directoryItems = [];

            return;
        }

        try {
            $adapter = $this->buildAdapter();
            $result = $adapter->files()->list($this->currentPath);
            $this->directoryItems = $result['items'] ?? [];
        } catch (Exception $e) {
            $this->directoryItems = [];
        }
    }

    // ── Step 4: Execute Restore ─────────────────────────────────────────

    public function executeRestore(): void
    {
        if ($this->restoreInProgress) {
            return;
        }

        $user = User::where('username', $this->selectedUser)->first();
        if (! $user) {
            Notification::make()->title(__('User not found'))->danger()->send();

            return;
        }

        $backup = $this->getBackup();
        if (! $backup) {
            Notification::make()->title(__('Backup not found'))->danger()->send();

            return;
        }

        $this->restoreInProgress = true;

        try {
            $orchestrator = app(BackupOrchestrator::class);
            $result = $orchestrator->restoreBackup($user, $backup, [
                'restore_files' => ! empty($this->selectedPaths) || ! empty($this->contents['domains']),
                'restore_databases' => ! empty($this->selectedDatabases),
                'restore_mailboxes' => ! empty($this->selectedMailboxes),
                'conflict_mode' => $this->conflictMode,
                'selected_domains' => ! empty($this->selectedPaths)
                    ? array_filter($this->selectedPaths, fn ($p) => ! str_contains($p, '/'))
                    : ($this->contents['domains'] ?? null),
                'selected_databases' => $this->selectedDatabases ?: null,
                'selected_mailboxes' => $this->selectedMailboxes ?: null,
                'selected_files' => array_filter($this->selectedPaths, fn ($p) => str_contains($p, '/')),
            ]);

            if ($result['success'] ?? false) {
                Notification::make()
                    ->title(__('Restore completed'))
                    ->body(__(':files domain(s), :dbs database(s), :mail mailbox(es)', [
                        'files' => $result['result']['files_count'] ?? 0,
                        'dbs' => $result['result']['databases_count'] ?? 0,
                        'mail' => $result['result']['mailboxes_count'] ?? 0,
                    ]))
                    ->success()
                    ->send();

                $this->redirect('/jabali-admin/backups');
            } else {
                Notification::make()
                    ->title(__('Restore failed'))
                    ->body($result['error'] ?? __('Unknown error'))
                    ->danger()
                    ->send();
            }
        } catch (Exception $e) {
            Notification::make()
                ->title(__('Restore failed'))
                ->body(SafeError::message($e))
                ->danger()
                ->send();
        } finally {
            $this->restoreInProgress = false;
        }
    }

    // ── Helpers ──────────────────────────────────────────────────────────

    private function buildAdapter(): BackupSnapshotAdapter
    {
        $backup = $this->getBackup();
        if (! $backup) {
            throw new Exception('Backup not found');
        }

        $repo = $backup->destination
            ? $backup->destination->getResticRepoUrl()
            : BackupDestination::defaultRepo();
        $destConfig = $backup->destination
            ? array_merge($backup->destination->config ?? [], ['type' => $backup->destination->type])
            : [];

        return new BackupSnapshotAdapter(
            app(AgentClient::class),
            $backup->snapshot_id,
            $this->selectedUser ?? 'admin',
            $repo,
            $destConfig,
        );
    }

    private function getBackup(): ?Backup
    {
        return Backup::find($this->backupId);
    }
}
