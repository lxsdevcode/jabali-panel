<div>
    <x-tab-loading-skeleton />
    <div wire:loading.remove wire:target="activeTab">
        @livewire(\App\Filament\Admin\Pages\WhmMigration::class, [], key('migration-whm'))
    </div>
</div>
