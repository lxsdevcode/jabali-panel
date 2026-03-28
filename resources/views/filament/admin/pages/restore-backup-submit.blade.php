<x-filament::button
    color="danger"
    icon="heroicon-o-arrow-path"
    wire:click="executeRestore"
    wire:confirm="{{ __('Are you sure you want to restore these items? This cannot be undone.') }}"
>
    {{ __('Restore Now') }}
</x-filament::button>
