<div>
    <x-tab-loading-skeleton />
    <div wire:loading.remove wire:target="activeTab">
        @livewire(\App\Filament\Admin\Pages\CpanelMigration::class, [], key('migration-cpanel'))
    </div>
</div>
