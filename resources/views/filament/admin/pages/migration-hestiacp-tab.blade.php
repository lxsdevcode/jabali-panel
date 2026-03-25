<div>
    <x-tab-loading-skeleton />
    <div wire:loading.remove wire:target="activeTab">
        @livewire(\App\Filament\Admin\Pages\HestiaCpMigration::class, [], key('migration-hestiacp'))
    </div>
</div>
