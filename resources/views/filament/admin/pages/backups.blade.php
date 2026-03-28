<x-filament-panels::page>
    <div class="flex gap-2 border-b border-gray-200 dark:border-white/10 pb-2 mb-4">
        <x-filament::button
            :color="$activeTab === 'backups' ? 'primary' : 'gray'"
            size="sm"
            icon="heroicon-o-cloud-arrow-up"
            wire:click="$set('activeTab', 'backups')"
        >
            {{ __('Backups') }}
        </x-filament::button>
        <x-filament::button
            :color="$activeTab === 'destinations' ? 'primary' : 'gray'"
            size="sm"
            icon="heroicon-o-server-stack"
            wire:click="$set('activeTab', 'destinations')"
        >
            {{ __('Destinations') }}
        </x-filament::button>
        <x-filament::button
            :color="$activeTab === 'schedules' ? 'primary' : 'gray'"
            size="sm"
            icon="heroicon-o-clock"
            wire:click="$set('activeTab', 'schedules')"
        >
            {{ __('Schedules') }}
        </x-filament::button>
    </div>

    {{ $this->table }}

    <x-filament-actions::modals />
</x-filament-panels::page>
