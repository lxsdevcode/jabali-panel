<x-filament-panels::page>
    {{-- Backup info --}}
    <x-filament::section>
        <x-slot name="heading">{{ __('Backup Information') }}</x-slot>
        @php $backup = $this->getBackup(); @endphp
        @if($backup)
            <div class="flex flex-wrap gap-4 text-sm">
                <div><span class="font-medium">{{ __('Name') }}:</span> {{ $backup->name }}</div>
                <div><span class="font-medium">{{ __('Date') }}:</span> {{ $backup->created_at?->format('M j, Y H:i') }}</div>
                <div><span class="font-medium">{{ __('Type') }}:</span> {{ ucfirst($backup->metadata['backup_type'] ?? $backup->type) }}</div>
                @if($backup->destination)
                    <div><span class="font-medium">{{ __('Destination') }}:</span> {{ $backup->destination->name }}</div>
                @endif
                @if(!$backup->local_path || !file_exists($backup->local_path))
                    <x-filament::badge color="info" icon="heroicon-o-cloud-arrow-down">{{ __('Remote — will download before restoring') }}</x-filament::badge>
                @endif
            </div>
        @endif
    </x-filament::section>

    {{-- User selector --}}
    <x-filament::section>
        <x-slot name="heading">{{ __('User to Restore') }}</x-slot>
        <div class="max-w-md">
            <select wire:model.live="restoreUsername" aria-label="{{ __('User to Restore') }}" class="fi-select-input w-full rounded-lg border-gray-300 bg-white text-gray-950 shadow-sm dark:border-white/10 dark:bg-white/5 dark:text-white">
                <option value="">{{ __('Select a user...') }}</option>
                @foreach($this->getUsers() as $value => $label)
                    <option value="{{ $value }}">{{ $label }}</option>
                @endforeach
            </select>
        </div>
    </x-filament::section>

    @if($restoreUsername)
        @if($this->isIncremental())
            {{-- Incremental backup: granular restore options --}}
            <x-filament::section>
                <x-slot name="heading">{{ __('Restore Options') }}</x-slot>
                <x-slot name="description">{{ __('Select what to restore from this incremental backup.') }}</x-slot>

                <div class="space-y-4">
                    {{-- Website Files --}}
                    <label class="flex items-center gap-3 cursor-pointer">
                        <input type="checkbox" wire:model.live="restoreFiles" class="fi-checkbox-input rounded border-gray-300 text-primary-600 shadow-sm dark:border-white/10 dark:bg-white/5">
                        <span class="font-medium">{{ __('Website Files') }}</span>
                    </label>

                    @if($restoreFiles && $restoreUsername !== '__all__')
                        <div class="ml-8 space-y-3">
                            <div class="flex gap-4">
                                <label class="flex items-center gap-2 cursor-pointer">
                                    <input type="radio" wire:model.live="filesRestoreMode" value="domains" class="fi-radio-input text-primary-600 border-gray-300 dark:border-white/10 dark:bg-white/5">
                                    {{ __('Full Domains') }}
                                </label>
                                <label class="flex items-center gap-2 cursor-pointer">
                                    <input type="radio" wire:model.live="filesRestoreMode" value="files" class="fi-radio-input text-primary-600 border-gray-300 dark:border-white/10 dark:bg-white/5">
                                    {{ __('Specific Files / Folders') }}
                                </label>
                            </div>
                        </div>
                    @endif

                    {{-- Databases --}}
                    <label class="flex items-center gap-3 cursor-pointer">
                        <input type="checkbox" wire:model.live="restoreDatabases" class="fi-checkbox-input rounded border-gray-300 text-primary-600 shadow-sm dark:border-white/10 dark:bg-white/5">
                        <span class="font-medium">{{ __('Databases') }}</span>
                    </label>

                    {{-- MySQL Users --}}
                    <label class="flex items-center gap-3 cursor-pointer">
                        <input type="checkbox" wire:model.live="restoreMysqlUsers" class="fi-checkbox-input rounded border-gray-300 text-primary-600 shadow-sm dark:border-white/10 dark:bg-white/5">
                        <span class="font-medium">{{ __('MySQL Users') }}</span>
                    </label>

                    {{-- Mailboxes --}}
                    <label class="flex items-center gap-3 cursor-pointer">
                        <input type="checkbox" wire:model.live="restoreMailboxes" class="fi-checkbox-input rounded border-gray-300 text-primary-600 shadow-sm dark:border-white/10 dark:bg-white/5">
                        <span class="font-medium">{{ __('Mailboxes') }}</span>
                    </label>

                    {{-- SSL --}}
                    <label class="flex items-center gap-3 cursor-pointer">
                        <input type="checkbox" wire:model.live="restoreSsl" class="fi-checkbox-input rounded border-gray-300 text-primary-600 shadow-sm dark:border-white/10 dark:bg-white/5">
                        <span class="font-medium">{{ __('SSL Certificates') }}</span>
                    </label>

                    {{-- DNS --}}
                    <label class="flex items-center gap-3 cursor-pointer">
                        <input type="checkbox" wire:model.live="restoreDns" class="fi-checkbox-input rounded border-gray-300 text-primary-600 shadow-sm dark:border-white/10 dark:bg-white/5">
                        <span class="font-medium">{{ __('DNS Zones') }}</span>
                    </label>
                </div>
            </x-filament::section>
        @else
            {{-- Full (tar.gz) backup: restore everything --}}
            <x-filament::section>
                <x-slot name="heading">{{ __('Full Backup Restore') }}</x-slot>
                <x-slot name="description">{{ __('This is a full archive backup. All data for the selected user will be restored.') }}</x-slot>

                <x-filament::badge color="info" icon="heroicon-o-archive-box">
                    {{ __('Website files, databases, mailboxes, SSL, and DNS will all be restored.') }}
                </x-filament::badge>
            </x-filament::section>
        @endif

        {{-- File Browser (only in specific files mode) --}}
        @if($restoreFiles && $filesRestoreMode === 'files' && $restoreUsername !== '__all__')
            <x-filament::section>
                <x-slot name="heading">{{ __('Browse Snapshot') }}</x-slot>

                {{-- Breadcrumbs --}}
                <nav class="flex items-center gap-1 text-sm mb-3" aria-label="{{ __('Snapshot path') }}">
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
                            <span class="text-gray-500 dark:text-gray-400" aria-current="location">{{ $label }}</span>
                        @endif
                    @endforeach
                </nav>

                {{-- Selected files indicator --}}
                @if(count($selectedFiles) > 0)
                    <div class="flex items-center gap-3 mb-3">
                        <x-filament::badge color="success">
                            {{ __(':count file(s)/folder(s) selected', ['count' => count($selectedFiles)]) }}
                        </x-filament::badge>
                        <x-filament::button wire:click="clearSelectedFiles" color="gray" size="sm">
                            {{ __('Clear Selection') }}
                        </x-filament::button>
                    </div>
                @endif

                {{-- Table --}}
                {{ $this->table }}
            </x-filament::section>
        @endif

        {{-- Restore button --}}
        <div class="flex gap-3">
            <x-filament::button wire:click="startRestore" color="warning" icon="heroicon-o-arrow-path" size="lg"
                wire:confirm="{{ __('Are you sure? Existing data may be overwritten.') }}">
                {{ __('Start Restore') }}
            </x-filament::button>
            <x-filament::button tag="a" href="{{ route('filament.admin.pages.backups', ['tab' => 'backups']) }}" color="gray" size="lg">
                {{ __('Cancel') }}
            </x-filament::button>
        </div>
    @endif

    <x-filament-actions::modals />
</x-filament-panels::page>
