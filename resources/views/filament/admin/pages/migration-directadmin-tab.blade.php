<div>
    <x-tab-loading-skeleton />
    <div wire:loading.remove wire:target="activeTab">
        @livewire(\App\Filament\Admin\Pages\DirectAdminMigration::class, [], key('migration-directadmin'))
    </div>
</div>
