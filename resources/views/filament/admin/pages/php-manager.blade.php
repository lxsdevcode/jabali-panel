<x-filament-panels::page>
    {{ $this->statsForm }}

    <x-filament::tabs>
        <x-filament::tabs.item
            :active="$activeTab === 'versions'"
            icon="heroicon-o-code-bracket"
            wire:click="$set('activeTab', 'versions')"
        >
            {{ __('PHP Versions') }}
        </x-filament::tabs.item>

        <x-filament::tabs.item
            :active="$activeTab === 'extensions'"
            icon="heroicon-o-puzzle-piece"
            wire:click="$set('activeTab', 'extensions')"
        >
            {{ __('PHP Extensions') }}
        </x-filament::tabs.item>
    </x-filament::tabs>

    @if ($activeTab === 'versions')
        {{ $this->table }}
    @endif

    @if ($activeTab === 'extensions')
        <x-filament::section icon="heroicon-o-puzzle-piece" icon-color="primary">
            <x-slot name="heading">{{ __('PHP Extensions') }}</x-slot>
            <x-slot name="description">{{ __('Manage installed extensions per PHP version') }}</x-slot>

            {{ $this->extensionVersionForm }}

            @if ($selectedExtensionVersion)
                <div class="mt-4">
                    @livewire('admin.php-extensions', ['version' => $selectedExtensionVersion], key('ext-'.$selectedExtensionVersion))
                </div>
            @endif
        </x-filament::section>
    @endif

    <x-filament-actions::modals />
</x-filament-panels::page>
