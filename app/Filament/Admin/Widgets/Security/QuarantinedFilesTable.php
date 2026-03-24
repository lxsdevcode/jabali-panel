<?php

declare(strict_types=1);

namespace App\Filament\Admin\Widgets\Security;

use App\Services\Agent\InteractsWithAgent;
use Filament\Actions\Action;
use Filament\Actions\Concerns\InteractsWithActions;
use Filament\Actions\Contracts\HasActions;
use Filament\Schemas\Concerns\InteractsWithSchemas;
use Filament\Schemas\Contracts\HasSchemas;
use Filament\Tables\Columns\TextColumn;
use Filament\Tables\Concerns\InteractsWithTable;
use Filament\Tables\Contracts\HasTable;
use Filament\Tables\Table;
use Livewire\Component;

class QuarantinedFilesTable extends Component implements HasActions, HasSchemas, HasTable
{
    use InteractsWithActions;
    use InteractsWithAgent;
    use InteractsWithSchemas;
    use InteractsWithTable;

    public array $files = [];

    public function makeFilamentTranslatableContentDriver(): ?\Filament\Support\Contracts\TranslatableContentDriver
    {
        return null;
    }

    public function table(Table $table): Table
    {
        return $table
            ->records(fn () => $this->files)
            ->columns([
                TextColumn::make('name')
                    ->label(__('File Name'))
                    ->icon('heroicon-o-document')
                    ->searchable()
                    ->limit(40),
                TextColumn::make('size')
                    ->label(__('Size'))
                    ->formatStateUsing(fn ($state): string => $state ? number_format($state / 1024, 1).' KB' : '-'),
                TextColumn::make('date')
                    ->label(__('Quarantined'))
                    ->date('M d, Y H:i'),
            ])
            ->actions([
                Action::make('delete')
                    ->label(__('Delete'))
                    ->icon('heroicon-o-trash')
                    ->color('danger')
                    ->requiresConfirmation()
                    ->modalHeading(__('Delete Quarantined File'))
                    ->modalDescription(__('Are you sure you want to permanently delete this file? This action cannot be undone.'))
                    ->action(function (array $record): void {
                        $this->agentCall(
                            action: 'clamav.delete_quarantined',
                            params: ['filename' => $record['name']],
                            successTitle: 'File Deleted',
                            errorTitle: 'Error',
                            onSuccess: fn () => $this->dispatch('refresh-security-data'),
                        );
                    }),
            ])
            ->striped()
            ->emptyStateHeading(__('No quarantined files'))
            ->emptyStateDescription(__('No files are currently in quarantine.'))
            ->emptyStateIcon('heroicon-o-archive-box');
    }

    public function render()
    {
        return $this->getTable()->render();
    }
}
