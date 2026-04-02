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

            <div class="mb-3">
                <select
                    wire:change="loadExtensions($event.target.value)"
                    class="fi-select-input block rounded-lg border-gray-300 shadow-sm transition duration-75 focus:border-primary-500 focus:ring-1 focus:ring-inset focus:ring-primary-500 dark:border-gray-600 dark:bg-gray-700 dark:text-white text-sm w-40"
                >
                    <option value="">{{ __('Select PHP Version') }}</option>
                    @foreach ($installedVersions as $v)
                        <option value="{{ $v['version'] }}" @selected($selectedExtensionVersion === $v['version'])>
                            PHP {{ $v['version'] }}
                        </option>
                    @endforeach
                </select>
            </div>

            @if ($selectedExtensionVersion)
                @livewire('admin.php-extensions', ['version' => $selectedExtensionVersion], key('ext-'.$selectedExtensionVersion))
            @endif
        </x-filament::section>
    @endif

    <x-filament-actions::modals />
</x-filament-panels::page>
