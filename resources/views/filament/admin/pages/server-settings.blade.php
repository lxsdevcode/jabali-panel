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
                        <input type="file" wire:model="logoLightUpload" accept="image/png,image/jpeg,image/webp,image/svg+xml" class="block w-full text-sm text-gray-500 file:mr-2 file:rounded file:border-0 file:bg-gray-100 file:px-3 file:py-1.5 file:text-sm file:font-medium file:text-gray-700 dark:text-gray-400 dark:file:bg-gray-700 dark:file:text-gray-300">
                        <div wire:loading wire:target="logoLightUpload" class="text-xs text-primary-500">{{ __('Uploading...') }}</div>
                        @error('logoLightUpload') <p class="text-xs text-danger-500">{{ $message }}</p> @enderror
                        <x-filament::button wire:click="saveLogoLight" icon="heroicon-o-photo" color="gray" size="sm" class="w-full" wire:loading.attr="disabled" wire:target="logoLightUpload">
                            {{ __('Save Light Logo') }}
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
                        <input type="file" wire:model="logoDarkUpload" accept="image/png,image/jpeg,image/webp,image/svg+xml" class="block w-full text-sm text-gray-500 file:mr-2 file:rounded file:border-0 file:bg-gray-100 file:px-3 file:py-1.5 file:text-sm file:font-medium file:text-gray-700 dark:text-gray-400 dark:file:bg-gray-700 dark:file:text-gray-300">
                        <div wire:loading wire:target="logoDarkUpload" class="text-xs text-primary-500">{{ __('Uploading...') }}</div>
                        @error('logoDarkUpload') <p class="text-xs text-danger-500">{{ $message }}</p> @enderror
                        <x-filament::button wire:click="saveLogoDark" icon="heroicon-o-moon" color="gray" size="sm" class="w-full" wire:loading.attr="disabled" wire:target="logoDarkUpload">
                            {{ __('Save Dark Logo') }}
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
