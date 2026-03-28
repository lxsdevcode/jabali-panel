<x-filament-panels::page>
    <x-filament::tabs>
        <x-filament::tabs.item
            :active="$activeTab === 'backups'"
            icon="heroicon-o-cloud-arrow-up"
            wire:click="$set('activeTab', 'backups')"
        >
            {{ __('Backups') }}
        </x-filament::tabs.item>

        <x-filament::tabs.item
            :active="$activeTab === 'destinations'"
            icon="heroicon-o-server-stack"
            wire:click="$set('activeTab', 'destinations')"
        >
            {{ __('Destinations') }}
        </x-filament::tabs.item>

        <x-filament::tabs.item
            :active="$activeTab === 'schedules'"
            icon="heroicon-o-clock"
            wire:click="$set('activeTab', 'schedules')"
        >
            {{ __('Schedules') }}
        </x-filament::tabs.item>

        <x-filament::tabs.item
            :active="$activeTab === 'logs'"
            icon="heroicon-o-document-text"
            wire:click="$set('activeTab', 'logs')"
        >
            {{ __('Logs') }}
        </x-filament::tabs.item>
    </x-filament::tabs>

    {{ $this->table }}

    <x-filament-actions::modals />
</x-filament-panels::page>
