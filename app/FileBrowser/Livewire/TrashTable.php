<?php

declare(strict_types=1);

namespace App\FileBrowser\Livewire;

use App\FileBrowser\Services\TrashManager;
use App\FileBrowser\Support\SafeError;
use Exception;
use Filament\Actions\Action;
use Filament\Actions\Concerns\InteractsWithActions;
use Filament\Actions\Contracts\HasActions;
use Filament\Notifications\Notification;
use Filament\Schemas\Concerns\InteractsWithSchemas;
use Filament\Schemas\Contracts\HasSchemas;
use Filament\Tables\Columns\TextColumn;
use Filament\Tables\Concerns\InteractsWithTable;
use Filament\Tables\Contracts\HasTable;
use Filament\Tables\Table;
use Illuminate\Database\Eloquent\Model;
use Livewire\Component;

class TrashTable extends Component implements HasActions, HasSchemas, HasTable
{
    use InteractsWithActions;
    use InteractsWithSchemas;
    use InteractsWithTable;

    protected static string $view = 'file-browser::components.trash-table';

    public array $trashItems = [];

    public function mount(): void
    {
        $this->loadTrashItems();
    }

    protected function getTrashManager(): TrashManager
    {
        return app(TrashManager::class);
    }

    public function loadTrashItems(): void
    {
        try {
            $this->trashItems = $this->getTrashManager()->items();
        } catch (Exception) {
            $this->trashItems = [];
        }
    }

    public function table(Table $table): Table
    {
        return $table
            ->records(fn () => $this->trashItems)
            ->columns([
                TextColumn::make('name')
                    ->label(__('Name'))
                    ->icon(fn (array $record): string => ($record['is_dir'] ?? false) ? 'heroicon-o-folder' : 'heroicon-o-document')
                    ->iconColor(fn (array $record): string => ($record['is_dir'] ?? false) ? 'warning' : 'primary')
                    ->weight('medium')
                    ->searchable(),
                TextColumn::make('trashed_at')
                    ->label(__('Deleted'))
                    ->formatStateUsing(fn (array $record): string => isset($record['trashed_at']) ? date('M d, Y H:i', $record['trashed_at']) : '')
                    ->color('gray'),
            ])
            ->recordActions([
                Action::make('restore')
                    ->label(__('Restore'))
                    ->icon('heroicon-o-arrow-uturn-left')
                    ->color('success')
                    ->action(function (array $record): void {
                        $this->restoreItem($record['trash_name']);
                    }),
                Action::make('delete')
                    ->label(__('Delete'))
                    ->icon('heroicon-o-x-mark')
                    ->color('danger')
                    ->requiresConfirmation()
                    ->modalHeading(__('Delete Permanently'))
                    ->modalDescription(__('This item will be permanently deleted. This cannot be undone.'))
                    ->modalSubmitActionLabel(__('Delete'))
                    ->action(function (array $record): void {
                        $this->deleteItem($record['trash_name']);
                    }),
            ])
            ->headerActions([
                Action::make('emptyTrash')
                    ->label(__('Empty Trash'))
                    ->icon('heroicon-o-trash')
                    ->color('danger')
                    ->requiresConfirmation()
                    ->modalHeading(__('Empty Trash'))
                    ->modalDescription(__('All items in trash will be permanently deleted. This cannot be undone.'))
                    ->modalSubmitActionLabel(__('Empty Trash'))
                    ->visible(fn () => count($this->trashItems) > 0)
                    ->action(function (): void {
                        $this->emptyTrash();
                    }),
                Action::make('refresh')
                    ->label(__('Refresh'))
                    ->icon('heroicon-o-arrow-path')
                    ->color('gray')
                    ->action(function (): void {
                        $this->loadTrashItems();
                        $this->resetTable();
                    }),
            ])
            ->emptyStateHeading(__('Trash is empty'))
            ->emptyStateDescription(__('Deleted items will appear here'))
            ->emptyStateIcon('heroicon-o-trash')
            ->striped();
    }

    public function getTableRecordKey(Model|array $record): string
    {
        return is_array($record) ? ($record['trash_name'] ?? $record['name'] ?? '') : $record->getKey();
    }

    public function restoreItem(string $trashName): void
    {
        try {
            $result = $this->getTrashManager()->restore($trashName);
            Notification::make()
                ->title(__('Restored'))
                ->body(__('Restored to: :path', ['path' => $result->restoredPath()]))
                ->success()
                ->send();
            $this->loadTrashItems();
            $this->resetTable();
            $this->dispatch('trash-updated');
        } catch (Exception $e) {
            Notification::make()
                ->title(__('Error'))
                ->body(SafeError::message($e))
                ->danger()
                ->send();
        }
    }

    public function deleteItem(string $trashName): void
    {
        try {
            $this->getTrashManager()->deletePermanently($trashName);
            Notification::make()
                ->title(__('Permanently deleted'))
                ->success()
                ->send();
            $this->loadTrashItems();
            $this->resetTable();
        } catch (Exception $e) {
            Notification::make()
                ->title(__('Error'))
                ->body(SafeError::message($e))
                ->danger()
                ->send();
        }
    }

    public function emptyTrash(): void
    {
        try {
            $result = $this->getTrashManager()->empty();
            Notification::make()
                ->title(__('Trash emptied'))
                ->body(__(':count items deleted', ['count' => $result->deletedCount()]))
                ->success()
                ->send();
            $this->loadTrashItems();
            $this->resetTable();
            $this->dispatch('trash-updated');
        } catch (Exception $e) {
            Notification::make()
                ->title(__('Error'))
                ->body(SafeError::message($e))
                ->danger()
                ->send();
        }
    }

    public function render()
    {
        return view(static::$view);
    }
}
