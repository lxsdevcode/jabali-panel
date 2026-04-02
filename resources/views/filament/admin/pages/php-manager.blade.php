<x-filament-panels::page>
    {{ $this->statsForm }}

    {{ $this->table }}

    {{-- Extension Management --}}
    <x-filament::section icon="heroicon-o-puzzle-piece" icon-color="primary">
        <x-slot name="heading">{{ __('PHP Extensions') }}</x-slot>
        <x-slot name="description">{{ __('Manage installed extensions per PHP version') }}</x-slot>

        <div x-data="{ search: '' }">
        <div class="flex items-center gap-3 mb-3">
            <select
                wire:change="loadExtensions($event.target.value)"
                class="fi-select-input block rounded-lg border-gray-300 shadow-sm transition duration-75 focus:border-primary-500 focus:ring-1 focus:ring-inset focus:ring-primary-500 dark:border-gray-600 dark:bg-gray-700 dark:text-white text-sm w-36"
            >
                <option value="">{{ __('Version') }}</option>
                @foreach ($installedVersions as $v)
                    <option value="{{ $v['version'] }}" @selected($selectedExtensionVersion === $v['version'])>
                        PHP {{ $v['version'] }}
                    </option>
                @endforeach
            </select>

            @if ($selectedExtensionVersion && count($extensions) > 0)
                <div class="relative grow max-w-xs">
                    <x-filament::icon icon="heroicon-m-magnifying-glass" class="absolute start-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" />
                    <input
                        type="search"
                        x-model="search"
                        placeholder="{{ __('Search extensions...') }}"
                        class="fi-input block w-full rounded-lg border-gray-300 ps-9 text-sm shadow-sm transition duration-75 focus:border-primary-500 focus:ring-1 focus:ring-inset focus:ring-primary-500 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
                    />
                </div>
            @endif
        </div>

        @if ($selectedExtensionVersion && count($extensions) > 0)
            <div class="overflow-x-auto">
                <table class="fi-ta-table w-full table-auto divide-y divide-gray-200 text-start dark:divide-gray-700">
                    <thead>
                        <tr>
                            <th class="fi-ta-header-cell px-3 py-2 text-start text-xs font-semibold text-gray-950 dark:text-white">{{ __('Extension') }}</th>
                            <th class="fi-ta-header-cell px-3 py-2 text-start text-xs font-semibold text-gray-950 dark:text-white">{{ __('Status') }}</th>
                            <th class="fi-ta-header-cell px-3 py-2 text-end text-xs font-semibold text-gray-950 dark:text-white">{{ __('Actions') }}</th>
                        </tr>
                    </thead>
                    <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
                        @foreach ($extensions as $ext)
                            <tr
                                class="fi-ta-row transition hover:bg-gray-50 dark:hover:bg-white/5"
                                x-show="!search || '{{ $ext['name'] }}'.includes(search.toLowerCase())"
                            >
                                <td class="fi-ta-cell px-3 py-1.5 text-sm text-gray-950 dark:text-white font-medium">
                                    {{ $ext['name'] }}
                                </td>
                                <td class="fi-ta-cell px-3 py-1.5 text-sm">
                                    @if (! ($ext['installed'] ?? true))
                                        <x-filament::badge size="sm" color="gray">{{ __('Not Installed') }}</x-filament::badge>
                                    @elseif ($ext['enabled'])
                                        <x-filament::badge size="sm" color="success">{{ __('Enabled') }}</x-filament::badge>
                                    @else
                                        <x-filament::badge size="sm" color="warning">{{ __('Disabled') }}</x-filament::badge>
                                    @endif
                                </td>
                                <td class="fi-ta-cell px-3 py-1.5 text-sm text-end">
                                    <div class="flex items-center justify-end gap-1">
                                        @if (! ($ext['installed'] ?? true))
                                            <x-filament::button size="xs" color="primary" wire:click="installExtension('{{ $ext['name'] }}')">{{ __('Install') }}</x-filament::button>
                                        @elseif ($ext['enabled'])
                                            <x-filament::button size="xs" color="warning" wire:click="disableExtension('{{ $ext['name'] }}')" wire:confirm="{{ __('Disable :ext? This may break sites using it.', ['ext' => $ext['name']]) }}">{{ __('Disable') }}</x-filament::button>
                                            <x-filament::button size="xs" color="danger" wire:click="removeExtension('{{ $ext['name'] }}')" wire:confirm="{{ __('Remove :ext? This will uninstall the package.', ['ext' => $ext['name']]) }}">{{ __('Remove') }}</x-filament::button>
                                        @else
                                            <x-filament::button size="xs" color="success" wire:click="enableExtension('{{ $ext['name'] }}')">{{ __('Enable') }}</x-filament::button>
                                            <x-filament::button size="xs" color="danger" wire:click="removeExtension('{{ $ext['name'] }}')" wire:confirm="{{ __('Remove :ext? This will uninstall the package.', ['ext' => $ext['name']]) }}">{{ __('Remove') }}</x-filament::button>
                                        @endif
                                    </div>
                                </td>
                            </tr>
                        @endforeach
                    </tbody>
                </table>
            </div>
        @elseif ($selectedExtensionVersion)
            <p class="text-sm text-gray-500 dark:text-gray-400">{{ __('No extensions found for PHP :version', ['version' => $selectedExtensionVersion]) }}</p>
        @endif
        </div>
    </x-filament::section>

    <x-filament-actions::modals />
</x-filament-panels::page>
