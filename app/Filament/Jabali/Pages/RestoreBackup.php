<?php

declare(strict_types=1);

namespace App\Filament\Jabali\Pages;

use App\Models\UserRemoteBackup;
use App\Services\Agent\InteractsWithAgent;
use App\Services\Backup\GranularRestoreService;
use App\Support\SafeError;
use BackedEnum;
use Exception;
use Filament\Actions\Concerns\InteractsWithActions;
use Filament\Actions\Contracts\HasActions;
use Filament\Forms\Components\Checkbox;
use Filament\Forms\Components\CheckboxList;
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
use Illuminate\Support\Facades\Auth;
use Illuminate\Support\HtmlString;

class RestoreBackup extends Page implements HasActions, HasForms, HasTable
{
    use InteractsWithActions;
    use InteractsWithAgent;
    use InteractsWithForms;
    use InteractsWithTable;

    protected static string|BackedEnum|null $navigationIcon = 'heroicon-o-arrow-path';

    protected static bool $shouldRegisterNavigation = false;

    protected string $view = 'filament.jabali.pages.restore-backup';

    protected static ?string $slug = 'backups/restore';

    public ?int $selectedSnapshotId = null;

    public string $restoreMode = 'full';

    public bool $safetyBackup = true;

    /** @var array<string, bool> */
    public array $selectedCategories = [
        'files' => false,
        'databases' => false,
        'mysql_users' => false,
        'mailboxes' => false,
        'dns_zones' => false,
        'ssl_certificates' => false,
    ];

    /** @var array<string, list<string>> */
    public array $selectedItems = [
        'domains' => [],
        'databases' => [],
        'mailboxes' => [],
        'dns_zones' => [],
        'ssl_certificates' => [],
    ];

    public string $conflictFiles = 'overwrite';

    public string $conflictDatabases = 'overwrite';

    public string $conflictMysqlUsers = 'overwrite';

    public string $conflictMailboxes = 'merge';

    /** @var array<string, mixed>|null */
    public ?array $snapshotContents = null;

    public string $currentPath = '';

    /** @var list<string> */
    public array $selectedFiles = [];

    public string $filesRestoreMode = 'domains';

    public function getTitle(): string|Htmlable
    {
        return __('Restore from Backup');
    }

    public function mount(): void
    {
        $snapshots = $this->getSnapshots();
        if ($snapshots->isNotEmpty()) {
            $this->selectedSnapshotId = $snapshots->first()->id;
        }
    }

    protected function getUser()
    {
        return Auth::user();
    }

    protected function getSnapshots()
    {
        return UserRemoteBackup::forUser($this->getUser()->id)
            ->with('destination')
            ->orderByDesc('backup_date')
            ->get();
    }

    protected function getSelectedSnapshot(): ?UserRemoteBackup
    {
        if (! $this->selectedSnapshotId) {
            return null;
        }

        return UserRemoteBackup::where('id', $this->selectedSnapshotId)
            ->where('user_id', $this->getUser()->id)
            ->with('destination')
            ->first();
    }

    public function updatedSelectedSnapshotId(): void
    {
        $this->snapshotContents = null;
        $this->selectedItems = [
            'domains' => [],
            'databases' => [],
            'mailboxes' => [],
            'dns_zones' => [],
            'ssl_certificates' => [],
        ];
        $this->selectedFiles = [];
        $this->currentPath = '';
    }

    public function updatedRestoreMode(): void
    {
        $this->snapshotContents = null;
    }

    public function loadSnapshotContents(): void
    {
        $snapshot = $this->getSelectedSnapshot();
        if (! $snapshot) {
            return;
        }

        try {
            $service = app(GranularRestoreService::class);
            $this->snapshotContents = $service->browseSnapshot($this->getUser(), $snapshot);
        } catch (Exception $e) {
            Notification::make()
                ->title(__('Error loading snapshot'))
                ->body(SafeError::message($e))
                ->danger()
                ->send();
        }
    }

    public function restoreOptionsForm(Schema $schema): Schema
    {
        $snapshots = $this->getSnapshots();
        $snapshotOptions = [];
        foreach ($snapshots as $snap) {
            $snapshotOptions[$snap->id] = $snap->backup_date?->format('M j, Y H:i') ?? $snap->backup_name;
        }

        return $schema->schema([
            Select::make('selectedSnapshotId')
                ->label(__('Backup Snapshot'))
                ->options($snapshotOptions)
                ->searchable()
                ->live()
                ->required()
                ->helperText(__('Select the backup date to restore from')),

            Radio::make('restoreMode')
                ->label(__('Restore Mode'))
                ->options([
                    'full' => __('Full Account Restore'),
                    'selected' => __('Selected Items'),
                ])
                ->inline()
                ->live()
                ->visible(fn (): bool => filled($this->selectedSnapshotId)),

            Checkbox::make('safetyBackup')
                ->label(__('Create safety backup before restoring'))
                ->helperText(__('Recommended: creates a backup of current state so you can undo if needed'))
                ->default(true)
                ->visible(fn (): bool => filled($this->selectedSnapshotId)),
        ]);
    }

    public function selectedItemsForm(Schema $schema): Schema
    {
        $contents = $this->snapshotContents ?? [];

        return $schema->schema([
            CheckboxList::make('selectedItems.domains')
                ->label(__('Domains'))
                ->options(fn (): array => array_combine($contents['domains'] ?? [], $contents['domains'] ?? []))
                ->visible(fn (): bool => ! empty($contents['domains'])),

            Radio::make('filesRestoreMode')
                ->label(__('Files Mode'))
                ->options([
                    'domains' => __('Full Domains'),
                    'files' => __('Specific Files / Folders'),
                ])
                ->inline()
                ->live()
                ->visible(fn (): bool => ! empty($this->selectedItems['domains'])),

            CheckboxList::make('selectedItems.databases')
                ->label(__('Databases'))
                ->options(fn (): array => array_combine($contents['databases'] ?? [], $contents['databases'] ?? []))
                ->visible(fn (): bool => ! empty($contents['databases'])),

            CheckboxList::make('selectedItems.mailboxes')
                ->label(__('Mailboxes'))
                ->options(fn (): array => array_combine($contents['mailboxes'] ?? [], $contents['mailboxes'] ?? []))
                ->visible(fn (): bool => ! empty($contents['mailboxes'])),

            Checkbox::make('selectedCategories.mysql_users')
                ->label(__('Restore MySQL Users'))
                ->visible(fn (): bool => $contents['mysql_users'] ?? false),

            CheckboxList::make('selectedItems.dns_zones')
                ->label(__('DNS Zones'))
                ->options(fn (): array => array_combine($contents['dns_zones'] ?? [], $contents['dns_zones'] ?? []))
                ->visible(fn (): bool => ! empty($contents['dns_zones'])),

            CheckboxList::make('selectedItems.ssl_certificates')
                ->label(__('SSL Certificates'))
                ->options(fn (): array => array_combine($contents['ssl_certificates'] ?? [], $contents['ssl_certificates'] ?? []))
                ->visible(fn (): bool => ! empty($contents['ssl_certificates'])),
        ]);
    }

    public function conflictForm(Schema $schema): Schema
    {
        return $schema->schema([
            Radio::make('conflictFiles')
                ->label(__('Files'))
                ->options([
                    'overwrite' => __('Overwrite existing'),
                    'merge' => __('Merge (keep files not in backup)'),
                ])
                ->inline()
                ->visible(fn (): bool => ! empty($this->selectedItems['domains']) || ! empty($this->selectedFiles)),

            Radio::make('conflictDatabases')
                ->label(__('Databases'))
                ->options([
                    'overwrite' => __('Overwrite existing'),
                    'skip' => __('Skip if exists'),
                    'rename' => __('Rename to {db}_restored'),
                ])
                ->inline()
                ->visible(fn (): bool => ! empty($this->selectedItems['databases'])),

            Radio::make('conflictMysqlUsers')
                ->label(__('MySQL Users'))
                ->options([
                    'overwrite' => __('Overwrite existing'),
                    'skip' => __('Skip if exists'),
                ])
                ->inline()
                ->visible(fn (): bool => $this->selectedCategories['mysql_users'] ?? false),

            Radio::make('conflictMailboxes')
                ->label(__('Mailboxes'))
                ->options([
                    'overwrite' => __('Overwrite existing'),
                    'merge' => __('Merge (keep new emails)'),
                ])
                ->inline()
                ->visible(fn (): bool => ! empty($this->selectedItems['mailboxes'])),
        ]);
    }

    protected function getForms(): array
    {
        return [
            'restoreOptionsForm',
            'selectedItemsForm',
            'conflictForm',
        ];
    }

    // ─── File Browser ──────────────────────────────────────────────

    public function navigateTo(string $path): void
    {
        if (str_contains($path, '..') || str_starts_with($path, '/')) {
            return;
        }

        $this->currentPath = $path;
        $this->resetTable();
    }

    public function toggleFile(string $path): void
    {
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
                if ($this->filesRestoreMode !== 'files' || empty($this->selectedSnapshotId)) {
                    return [];
                }

                $snapshot = $this->getSelectedSnapshot();
                if (! $snapshot) {
                    return [];
                }

                try {
                    $service = app(GranularRestoreService::class);
                    $items = $service->browseDomainFiles($this->getUser(), $snapshot, $this->currentPath ?: 'domains');

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
                    ->formatStateUsing(fn ($state): string => $state ? \App\Support\Formatter::bytes((int) $state) : '-')
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
            ->emptyStateHeading(__('No files found'))
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
        $path = $this->currentPath ?: 'domains';
        $parts = explode('/', $path);
        $built = '';
        foreach ($parts as $part) {
            $built = $built ? $built.'/'.$part : $part;
            $crumbs[$built] = $part;
        }

        return $crumbs;
    }

    // ─── Restore Actions ───────────────────────────────────────────

    public function startRestore(): void
    {
        $snapshot = $this->getSelectedSnapshot();
        if (! $snapshot) {
            Notification::make()->title(__('Please select a snapshot'))->warning()->send();

            return;
        }

        $user = $this->getUser();
        $service = app(GranularRestoreService::class);

        try {
            if ($this->restoreMode === 'full') {
                $restore = $service->fullRestore($user, $snapshot, $this->safetyBackup);
            } else {
                $selection = $this->buildSelection();
                if (empty(array_filter($selection))) {
                    Notification::make()->title(__('Please select at least one item'))->warning()->send();

                    return;
                }

                $conflictResolution = $this->buildConflictResolution();
                $restore = $service->selectiveRestore($user, $snapshot, $selection, $conflictResolution);
            }

            if ($restore->status === 'completed') {
                $result = $restore->result ?? [];
                $failed = $result['failed'] ?? [];

                if (! empty($failed)) {
                    Notification::make()
                        ->title(__('Restore completed with warnings'))
                        ->body(__(':failed item(s) failed. Check restore history for details.', ['failed' => count($failed)]))
                        ->warning()
                        ->send();
                } else {
                    Notification::make()
                        ->title(__('Restore completed successfully'))
                        ->success()
                        ->send();
                }
            } else {
                Notification::make()
                    ->title(__('Restore failed'))
                    ->body($restore->error_message ?? __('Unknown error'))
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

    /**
     * @return array<string, mixed>
     */
    protected function buildSelection(): array
    {
        $selection = [];

        if (! empty($this->selectedItems['domains'])) {
            $selection['domains'] = $this->selectedItems['domains'];
        }

        if (! empty($this->selectedFiles)) {
            $selection['files'] = $this->selectedFiles;
        }

        if (! empty($this->selectedItems['databases'])) {
            $selection['databases'] = $this->selectedItems['databases'];
        }

        if (! empty($this->selectedItems['mailboxes'])) {
            $selection['mailboxes'] = $this->selectedItems['mailboxes'];
        }

        if ($this->selectedCategories['mysql_users'] ?? false) {
            $selection['mysql_users'] = true;
        }

        if (! empty($this->selectedItems['dns_zones'])) {
            $selection['dns_zones'] = $this->selectedItems['dns_zones'];
        }

        if (! empty($this->selectedItems['ssl_certificates'])) {
            $selection['ssl_certificates'] = $this->selectedItems['ssl_certificates'];
        }

        return $selection;
    }

    /**
     * @return array<string, string>
     */
    protected function buildConflictResolution(): array
    {
        $resolution = [];

        if (! empty($this->selectedItems['domains']) || ! empty($this->selectedFiles)) {
            $resolution['files'] = $this->conflictFiles;
        }
        if (! empty($this->selectedItems['databases'])) {
            $resolution['databases'] = $this->conflictDatabases;
        }
        if ($this->selectedCategories['mysql_users'] ?? false) {
            $resolution['mysql_users'] = $this->conflictMysqlUsers;
        }
        if (! empty($this->selectedItems['mailboxes'])) {
            $resolution['mailboxes'] = $this->conflictMailboxes;
        }

        return $resolution;
    }
}
