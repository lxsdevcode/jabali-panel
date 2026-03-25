<x-filament-panels::page>
    {{-- Snapshot Selection --}}
    <x-filament::section>
        <x-slot name="heading">{{ __('Select Backup') }}</x-slot>
        <x-slot name="description">{{ __('Choose a backup snapshot and restore mode.') }}</x-slot>

        {{ $this->restoreOptionsForm }}
    </x-filament::section>

    @if($selectedSnapshotId)
        @if($restoreMode === 'selected')
            {{-- Load contents button --}}
            @if(!$snapshotContents)
                <x-filament::section>
                    <x-slot name="heading">{{ __('Browse Snapshot Contents') }}</x-slot>
                    <x-filament::button wire:click="loadSnapshotContents" color="primary" icon="heroicon-o-magnifying-glass">
                        {{ __('Load Contents') }}
                    </x-filament::button>
                </x-filament::section>
            @else
                {{-- Item Selection --}}
                <x-filament::section>
                    <x-slot name="heading">{{ __('Select Items to Restore') }}</x-slot>
                    <x-slot name="description">{{ __('Check the items you want to restore from this snapshot.') }}</x-slot>

                    {{ $this->selectedItemsForm }}
                </x-filament::section>

                {{-- File Browser --}}
                @if($filesRestoreMode === 'files' && !empty($selectedItems['domains']))
                    <x-filament::section>
                        <x-slot name="heading">{{ __('Browse Files') }}</x-slot>

                        {{-- Breadcrumbs --}}
                        <nav class="flex items-center gap-1 text-sm mb-3">
                            @foreach($this->getBreadcrumbs() as $path => $label)
                                @if(!$loop->last)
                                    <button wire:click="navigateTo('{{ $path }}')" class="fi-link fi-link-size-sm text-primary-600 dark:text-primary-400 hover:underline">
                                        @if($loop->first)
                                            <x-filament::icon icon="heroicon-o-home" class="inline h-4 w-4" />
                                        @endif
                                        {{ $label }}
                                    </button>
                                    <x-filament::icon icon="heroicon-o-chevron-right" class="h-3 w-3 text-gray-400" />
                                @else
                                    <span class="text-gray-500 dark:text-gray-400">{{ $label }}</span>
                                @endif
                            @endforeach
                        </nav>

                        @if(count($selectedFiles) > 0)
                            <div class="flex items-center gap-3 mb-3">
                                <x-filament::badge color="success">
                                    {{ __(':count file(s)/folder(s) selected', ['count' => count($selectedFiles)]) }}
                                </x-filament::badge>
                                <x-filament::button wire:click="clearSelectedFiles" color="gray" size="sm">
                                    {{ __('Clear') }}
                                </x-filament::button>
                            </div>
                        @endif

                        {{ $this->table }}
                    </x-filament::section>
                @endif

                {{-- Conflict Resolution --}}
                @if(!empty($selectedItems['domains']) || !empty($selectedFiles) || !empty($selectedItems['databases']) || !empty($selectedItems['mailboxes']) || ($selectedCategories['mysql_users'] ?? false))
                    <x-filament::section>
                        <x-slot name="heading">{{ __('Conflict Resolution') }}</x-slot>
                        <x-slot name="description">{{ __('Choose how to handle existing data.') }}</x-slot>

                        {{ $this->conflictForm }}
                    </x-filament::section>
                @endif
            @endif
        @endif

        {{-- Restore Button --}}
        <div class="flex gap-3">
            <x-filament::button
                wire:click="startRestore"
                color="warning"
                icon="heroicon-o-arrow-path"
                size="lg"
                wire:confirm="{{ __('Are you sure? Existing data may be overwritten.') }}"
                :disabled="$restoreMode === 'selected' && !$snapshotContents"
            >
                {{ $restoreMode === 'full' ? __('Full Account Restore') : __('Restore Selected Items') }}
            </x-filament::button>

            <x-filament::button
                tag="a"
                href="{{ \App\Filament\Jabali\Pages\Backups::getUrl() }}"
                color="gray"
                size="lg"
            >
                {{ __('Back to Backups') }}
            </x-filament::button>
        </div>
    @endif

    <x-filament-actions::modals />
</x-filament-panels::page>
