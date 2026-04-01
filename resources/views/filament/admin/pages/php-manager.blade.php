<x-filament-panels::page>
    {{ $this->statsForm }}

    {{ $this->table }}

    {{-- Extension Management --}}
    <x-filament::section icon="heroicon-o-puzzle-piece" icon-color="primary">
        <x-slot name="heading">{{ __('PHP Extensions') }}</x-slot>
        <x-slot name="description">{{ __('Manage installed extensions per PHP version') }}</x-slot>

        <div class="mb-4">
            <select
                wire:change="loadExtensions($event.target.value)"
                class="fi-select-input block w-full rounded-lg border-gray-300 shadow-sm transition duration-75 focus:border-primary-500 focus:ring-1 focus:ring-inset focus:ring-primary-500 dark:border-gray-600 dark:bg-gray-700 dark:text-white sm:max-w-xs sm:text-sm"
            >
                <option value="">{{ __('Select PHP Version') }}</option>
                @foreach ($installedVersions as $v)
                    <option value="{{ $v['version'] }}" @selected($selectedExtensionVersion === $v['version'])>
                        PHP {{ $v['version'] }}
                    </option>
                @endforeach
            </select>
        </div>

        @if ($selectedExtensionVersion && count($extensions) > 0)
            <div class="overflow-x-auto">
                <table class="fi-ta-table w-full table-auto divide-y divide-gray-200 text-start dark:divide-gray-700">
                    <thead>
                        <tr>
                            <th class="fi-ta-header-cell px-3 py-3.5 text-start text-sm font-semibold text-gray-950 dark:text-white">{{ __('Extension') }}</th>
                            <th class="fi-ta-header-cell px-3 py-3.5 text-start text-sm font-semibold text-gray-950 dark:text-white">{{ __('Status') }}</th>
                            <th class="fi-ta-header-cell px-3 py-3.5 text-end text-sm font-semibold text-gray-950 dark:text-white">{{ __('Actions') }}</th>
                        </tr>
                    </thead>
                    <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
                        @foreach ($extensions as $ext)
                            <tr class="fi-ta-row transition hover:bg-gray-50 dark:hover:bg-white/5">
                                <td class="fi-ta-cell px-3 py-4 text-sm text-gray-950 dark:text-white font-medium">
                                    {{ $ext['name'] }}
                                </td>
                                <td class="fi-ta-cell px-3 py-4 text-sm">
                                    @if ($ext['enabled'])
                                        <x-filament::badge color="success">{{ __('Enabled') }}</x-filament::badge>
                                    @else
                                        <x-filament::badge color="gray">{{ __('Disabled') }}</x-filament::badge>
                                    @endif
                                </td>
                                <td class="fi-ta-cell px-3 py-4 text-sm text-end">
                                    <div class="flex items-center justify-end gap-2">
                                        @if ($ext['enabled'])
                                            <x-filament::button
                                                size="xs"
                                                color="warning"
                                                wire:click="disableExtension('{{ $ext['name'] }}')"
                                                wire:confirm="{{ __('Disable :ext? This may break sites using it.', ['ext' => $ext['name']]) }}"
                                            >
                                                {{ __('Disable') }}
                                            </x-filament::button>
                                        @else
                                            <x-filament::button
                                                size="xs"
                                                color="success"
                                                wire:click="enableExtension('{{ $ext['name'] }}')"
                                            >
                                                {{ __('Enable') }}
                                            </x-filament::button>
                                        @endif
                                        <x-filament::button
                                            size="xs"
                                            color="danger"
                                            wire:click="removeExtension('{{ $ext['name'] }}')"
                                            wire:confirm="{{ __('Remove :ext? This will uninstall the package.', ['ext' => $ext['name']]) }}"
                                        >
                                            {{ __('Remove') }}
                                        </x-filament::button>
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
    </x-filament::section>

    <x-filament-actions::modals />
</x-filament-panels::page>
