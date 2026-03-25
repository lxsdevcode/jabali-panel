<div>
    <x-tab-loading-skeleton />
    <div wire:loading.remove wire:target="activeTab">
        @livewire(\App\Filament\Jabali\Pages\ImapSync::class, [], key('migration-imap'))
    </div>
</div>
