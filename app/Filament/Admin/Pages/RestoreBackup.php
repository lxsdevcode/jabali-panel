<?php

declare(strict_types=1);

namespace App\Filament\Admin\Pages;

use App\Models\Backup;
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
use Filament\Forms\Components\Select;
use Filament\Forms\Concerns\InteractsWithForms;
use Filament\Forms\Contracts\HasForms;
use Filament\Notifications\Notification;
use Filament\Pages\Page;
use Illuminate\Contracts\Support\Htmlable;

class RestoreBackup extends Page implements HasActions, HasForms
{
    use InteractsWithActions;
    use InteractsWithForms;

    protected static ?string $slug = 'backups/restore/{backupId}';

    protected static bool $shouldRegisterNavigation = false;

    protected static string|BackedEnum|null $navigationIcon = 'heroicon-o-arrow-path';

    protected string $view = 'filament.admin.pages.restore-backup';

    public ?int $backupId = null;

    public string $currentPath = '';

    public array $selectedPaths = [];

    private ?Backup $backup = null;

    public function mount(?int $backupId = null): void
    {
        $this->backupId = $backupId ?? (int) request()->route('backupId');
        $this->backup = Backup::find($this->backupId);

        if (! $this->backup || ! $this->backup->snapshot_id) {
            $this->redirect('/jabali-admin/backups');
        }
    }

    public function getTitle(): string|Htmlable
    {
        return __('Restore: :name', ['name' => $this->getBackup()?->name ?? '']);
    }

    public function getSubheading(): ?string
    {
        $path = $this->currentPath ?: '/';

        return __('Browsing: :path — Click folders to navigate, click "Restore This Folder" to restore.', ['path' => $path]);
    }

    protected function getHeaderActions(): array
    {
        return [
            Action::make('restoreSelected')
                ->label(fn () => __('Restore Selected (:count)', ['count' => count($this->selectedPaths)]))
                ->icon('heroicon-o-arrow-path')
                ->color('danger')
                ->visible(fn () => count($this->selectedPaths) > 0)
                ->requiresConfirmation()
                ->modalHeading(__('Restore Selected Items'))
                ->modalDescription(fn () => __('Restore :count selected item(s)?', ['count' => count($this->selectedPaths)]))
                ->form([
                    Select::make('restore_user')
                        ->label(__('Restore for user'))
                        ->options(fn () => User::where('is_active', true)->pluck('username', 'username')->toArray())
                        ->required(),
                ])
                ->action(function (array $data): void {
                    $this->restoreSelectedPaths($data['restore_user']);
                }),
            Action::make('restoreCurrentFolder')
                ->label(__('Restore This Folder'))
                ->icon('heroicon-o-arrow-path')
                ->color('warning')
                ->requiresConfirmation()
                ->modalHeading(__('Restore Folder'))
                ->modalDescription(fn () => empty($this->currentPath)
                    ? __('Restore the entire user home directory?')
                    : __('Restore ":path"?', ['path' => $this->currentPath]))
                ->form([
                    Select::make('restore_user')
                        ->label(__('Restore for user'))
                        ->options(fn () => User::where('is_active', true)->pluck('username', 'username')->toArray())
                        ->required(),
                ])
                ->action(function (array $data): void {
                    $this->restorePath($data['restore_user']);
                }),
            Action::make('back')
                ->label(__('Back to Backups'))
                ->icon('heroicon-o-arrow-left')
                ->color('gray')
                ->url('/jabali-admin/backups'),
        ];
    }

    public function navigateTo(string $path): void
    {
        $this->currentPath = $path;
    }

    private function restoreSelectedPaths(string $username): void
    {
        $user = User::where('username', $username)->first();
        if (! $user) {
            Notification::make()->title(__('User not found'))->danger()->send();

            return;
        }

        $backup = $this->getBackup();

        try {
            $repo = $backup->destination
                ? $backup->destination->getResticRepoUrl()
                : '/var/backups/jabali/restic';
            $destConfig = $backup->destination
                ? array_merge($backup->destination->config ?? [], ['type' => $backup->destination->type])
                : [];

            // Build --include args for each selected path
            $includes = array_map(
                fn ($p) => "/home/{$username}/{$p}",
                $this->selectedPaths
            );

            $agent = app(AgentClient::class);
            $includeArgs = implode(' ', array_map(fn ($p) => '--include '.escapeshellarg($p), $includes));

            $result = $agent->send('backup.restore_paths', [
                'snapshot_id' => $backup->snapshot_id,
                'username' => $username,
                'repo' => $repo,
                'destination' => $destConfig,
                'include_paths' => $includes,
            ]);

            if ($result['success'] ?? false) {
                Notification::make()
                    ->title(__('Restore completed'))
                    ->body(__(':count item(s) restored', ['count' => count($this->selectedPaths)]))
                    ->success()
                    ->send();

                $this->selectedPaths = [];
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
        }
    }

    public function loadDirectory(): array
    {
        $backup = $this->getBackup();
        if (! $backup) {
            return [];
        }

        try {
            $adapter = $this->buildAdapter();
            $result = $adapter->files()->list($this->currentPath);

            return $result['items'] ?? [];
        } catch (Exception $e) {
            return [];
        }
    }

    private function buildAdapter(): BackupSnapshotAdapter
    {
        $backup = $this->getBackup();
        $repo = $backup->destination
            ? $backup->destination->getResticRepoUrl()
            : '/var/backups/jabali/restic';
        $destConfig = $backup->destination
            ? array_merge($backup->destination->config ?? [], ['type' => $backup->destination->type])
            : [];

        $username = $backup->users[0] ?? 'admin';

        return new BackupSnapshotAdapter(
            app(AgentClient::class),
            $backup->snapshot_id,
            $username,
            $repo,
            $destConfig,
        );
    }

    private function restorePath(string $username): void
    {
        $user = User::where('username', $username)->first();
        if (! $user) {
            Notification::make()->title(__('User not found'))->danger()->send();

            return;
        }

        $backup = $this->getBackup();

        try {
            $orchestrator = app(BackupOrchestrator::class);
            $result = $orchestrator->restoreBackup($user, $backup, [
                'restore_files' => true,
                'restore_databases' => false,
                'restore_mailboxes' => false,
            ]);

            if ($result['success'] ?? false) {
                Notification::make()
                    ->title(__('Restore completed'))
                    ->success()
                    ->send();
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
        }
    }

    private function getBackup(): ?Backup
    {
        if ($this->backup === null) {
            $this->backup = Backup::find($this->backupId);
        }

        return $this->backup;
    }
}
