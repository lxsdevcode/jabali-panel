<x-filament-panels::page>
    @if ($activeTab === 'general')
        <x-filament::section icon="heroicon-o-paint-brush">
            <x-slot name="heading">{{ __('Panel Branding') }}</x-slot>

            <div class="space-y-6">
                <div>
                    <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                        {{ __('Control Panel Name') }}
                    </label>
                    <input
                        type="text"
                        wire:model="brandingData.panel_name"
                        placeholder="{{ __('Jabali') }}"
                        class="fi-input mt-1 block w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-950 shadow-sm transition duration-75 focus:border-primary-500 focus:ring-1 focus:ring-primary-500 dark:border-gray-700 dark:bg-gray-900 dark:text-white"
                    >
                    <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ __('Appears in browser title and navigation') }}</p>
                </div>

                <div class="grid grid-cols-1 gap-6 md:grid-cols-2">
                    {{-- Light Logo --}}
                    <div class="space-y-3">
                        <p class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ __('Light Logo') }}</p>
                        @if ($this->currentLogo)
                            <div class="flex aspect-square max-h-32 items-center justify-center rounded-lg border border-gray-200 bg-white p-2 dark:border-gray-700 dark:bg-gray-900">
                                <img src="{{ asset('storage/' . $this->currentLogo) }}" alt="{{ __('Light Logo') }}" class="max-h-full max-w-full object-contain">
                            </div>
                        @else
                            <div class="flex aspect-square max-h-32 items-center justify-center rounded-lg border-2 border-dashed border-gray-300 bg-gray-50 dark:border-gray-600 dark:bg-gray-800">
                                <p class="text-xs text-gray-400">{{ __('No logo') }}</p>
                            </div>
                        @endif
                        <x-filament::button wire:click="openUploadLogoLight" icon="heroicon-o-photo" color="gray" size="sm" class="w-full">
                            {{ __('Upload Light Logo') }}
                        </x-filament::button>
                    </div>

                    {{-- Dark Logo --}}
                    <div class="space-y-3">
                        <p class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ __('Dark Logo') }}</p>
                        @if ($this->currentLogoDark)
                            <div class="flex aspect-square max-h-32 items-center justify-center rounded-lg border border-gray-200 bg-white p-2 dark:border-gray-700 dark:bg-gray-900">
                                <img src="{{ asset('storage/' . $this->currentLogoDark) }}" alt="{{ __('Dark Logo') }}" class="max-h-full max-w-full object-contain">
                            </div>
                        @else
                            <div class="flex aspect-square max-h-32 items-center justify-center rounded-lg border-2 border-dashed border-gray-300 bg-gray-50 dark:border-gray-600 dark:bg-gray-800">
                                <p class="text-xs text-gray-400">{{ __('No logo') }}</p>
                            </div>
                        @endif
                        <x-filament::button wire:click="openUploadLogoDark" icon="heroicon-o-moon" color="gray" size="sm" class="w-full">
                            {{ __('Upload Dark Logo') }}
                        </x-filament::button>
                    </div>
                </div>

                <div class="flex gap-3">
                    <x-filament::button wire:click="saveBranding">
                        {{ __('Save Branding') }}
                    </x-filament::button>
                    @if ($this->currentLogo || $this->currentLogoDark)
                        <x-filament::button wire:click="removeLogo" color="danger" icon="heroicon-o-trash" wire:confirm="{{ __('Are you sure you want to remove the logos?') }}">
                            {{ __('Remove Logos') }}
                        </x-filament::button>
                    @endif
                </div>
            </div>
        </x-filament::section>
    @endif

    {{ $this->settingsForm }}

    <x-filament-actions::modals />
</x-filament-panels::page>
