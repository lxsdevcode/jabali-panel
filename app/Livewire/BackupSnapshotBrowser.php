<?php

declare(strict_types=1);

namespace App\Livewire;

use App\Models\BackupDestination;
use App\Services\Agent\AgentClient;
use App\Services\FileBrowser\BackupSnapshotAdapter;
use Filament\Notifications\Notification;
use Filament\Tables\Columns\TextColumn;
use Filament\Tables\Concerns\InteractsWithTable;
use Filament\Tables\Contracts\HasTable;
use Filament\Tables\Table;
use Illuminate\Contracts\View\View;
use Illuminate\Database\Eloquent\Model;
use Jabali\FileBrowser\Support\Formatter;
use Livewire\Component;

/**
 * Embeddable backup snapshot file browser for the restore modal.
 * Read-only — browses remote backup snapshots via SSH.
 */
class BackupSnapshotBrowser extends Component implements \Filament\Actions\Contracts\HasActions, \Filament\Forms\Contracts\HasForms, HasTable
{
    use \Filament\Actions\Concerns\InteractsWithActions;
    use \Filament\Forms\Concerns\InteractsWithForms;
    use InteractsWithTable;

    public string $backupPath = '';

    public string $username = '';

    public int $destinationId = 0;

    public string $currentPath = '';

    public array $selectedFiles = [];

    public function mount(string $backupPath = '', string $username = '', int $destinationId = 0): void
    {
        $this->backupPath = $backupPath;
        $this->username = $username;
        $this->destinationId = $destinationId;
    }

    public function navigateTo(string $path): void
    {
        $this->currentPath = $path;
        $this->resetTable();
    }

    public function toggleFileSelection(string $path): void
    {
        if (in_array($path, $this->selectedFiles)) {
            $this->selectedFiles = array_values(array_diff($this->selectedFiles, [$path]));
        } else {
            $this->selectedFiles[] = $path;
        }

        $this->dispatch('snapshot-files-selected', files: $this->selectedFiles);
    }

    public function table(Table $table): Table
    {
        return $table
            ->records(function (): array {
                if (empty($this->backupPath) || empty($this->username) || empty($this->destinationId)) {
                    return [];
                }

                $destination = BackupDestination::find($this->destinationId);
                if (! $destination) {
                    return [];
                }

                try {
                    $config = array_merge($destination->config ?? [], ['type' => $destination->type]);
                    $adapter = new BackupSnapshotAdapter(
                        app(AgentClient::class),
                        $this->backupPath,
                        $this->username,
                        $config,
                    );

                    $result = $adapter->files()->list($this->currentPath);
                    $items = $result['items'] ?? [];

                    // Add sequential IDs
                    $indexed = [];
                    foreach ($items as $i => $item) {
                        $item['id'] = $i + 1;
                        $indexed[$i + 1] = $item;
                    }

                    return $indexed;
                } catch (\Throwable $e) {
                    Notification::make()
                        ->title(__('Error loading directory'))
                        ->body($e->getMessage())
                        ->danger()
                        ->send();

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
                    ->formatStateUsing(function (string $state, array $record): \Illuminate\Support\HtmlString {
                        $name = e($state);
                        if ($record['is_dir'] ?? false) {
                            $path = e($record['path'] ?? '');

                            return new \Illuminate\Support\HtmlString(
                                "<button type=\"button\" wire:click=\"navigateTo('{$path}')\" class=\"text-left hover:underline cursor-pointer\">{$name}</button>"
                            );
                        }

                        return new \Illuminate\Support\HtmlString($name);
                    })
                    ->html()
                    ->searchable(isIndividual: true),
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
            ->checkIfRecordIsSelectableUsing(fn (array $record): bool => ! ($record['is_parent'] ?? false))
            ->resolveSelectedRecordsUsing(function (array $keys): array {
                // Re-fetch current directory items and return only selected ones
                if (empty($this->backupPath) || empty($this->username) || empty($this->destinationId)) {
                    return [];
                }

                $destination = BackupDestination::find($this->destinationId);
                if (! $destination) {
                    return [];
                }

                try {
                    $config = array_merge($destination->config ?? [], ['type' => $destination->type]);
                    $adapter = new BackupSnapshotAdapter(
                        app(AgentClient::class),
                        $this->backupPath,
                        $this->username,
                        $config,
                    );
                    $result = $adapter->files()->list($this->currentPath);
                    $all = [];
                    foreach ($result['items'] ?? [] as $i => $item) {
                        $item['id'] = $i + 1;
                        $key = $item['path'] ?? (string) ($i + 1);
                        $all[$key] = $item;
                    }

                    return array_intersect_key($all, array_flip($keys));
                } catch (\Throwable) {
                    return [];
                }
            })
            ->toolbarActions([
                \Filament\Actions\BulkAction::make('confirmSelection')
                    ->label(__('Use Selected for Restore'))
                    ->icon('heroicon-o-check')
                    ->color('success')
                    ->deselectRecordsAfterCompletion()
                    ->action(function (\Illuminate\Support\Collection $records): void {
                        $paths = $records->map(fn (array $r): string => $r['path'] ?? '')->filter()->values()->toArray();
                        $this->selectedFiles = $paths;
                        $this->dispatch('snapshot-files-selected', files: $this->selectedFiles);

                        Notification::make()
                            ->title(__(':count item(s) selected for restore', ['count' => count($this->selectedFiles)]))
                            ->success()
                            ->send();
                    }),
            ])
            ->paginated(false)
            ->emptyStateHeading(__('This folder is empty'))
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

    public function render(): View
    {
        return view('livewire.backup-snapshot-browser', [
            'breadcrumbs' => $this->getBreadcrumbs(),
        ]);
    }
}
