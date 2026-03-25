<div>
    <x-tab-loading-skeleton />
    <div wire:loading.remove wire:target="activeTab">
        @livewire(\App\Filament\Jabali\Pages\DirectAdminMigration::class, [], key('migration-directadmin'))
    </div>
</div>
