<x-filament-panels::page>
    {{-- Step indicators --}}
    <div class="flex gap-2 mb-6">
        @foreach([1 => 'Account', 2 => 'Contents', 3 => 'Select Items', 4 => 'Confirm'] as $num => $label)
            <button wire:key="step-{{ $num }}"
                wire:click="goToStep({{ $num }})"
                @class([
                    'flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium transition',
                    'bg-primary-500 text-white' => $this->step === $num,
                    'bg-gray-100 text-gray-600 dark:bg-white/5 dark:text-gray-400' => $this->step !== $num && $num < $this->step,
                    'bg-gray-50 text-gray-400 dark:bg-white/5 dark:text-gray-500 cursor-not-allowed' => $num > $this->step,
                ])
                @if($num > $this->step) disabled @endif
            >
                <span class="flex items-center justify-center w-6 h-6 rounded-full text-xs {{ $this->step === $num ? 'bg-white/20' : 'bg-gray-200 dark:bg-white/10' }}">{{ $num }}</span>
                {{ __($label) }}
            </button>
        @endforeach
    </div>

    {{-- Step 1: Select Account --}}
    @if($this->step === 1)
        <x-filament::section>
            <x-slot name="heading">{{ __('Select Account & Backup') }}</x-slot>

            <div class="space-y-4">
                <div>
                    <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{ __('Restore for user') }}</label>
                    <select wire:model.live="selectedUser" class="w-full rounded-lg border-gray-300 dark:border-white/10 dark:bg-white/5 dark:text-white">
                        <option value="">{{ __('Select a user...') }}</option>
                        @foreach($this->activeUsers as $username)
                            <option value="{{ $username }}">{{ $username }}</option>
                        @endforeach
                    </select>
                </div>

                <div>
                    <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{ __('Backup') }}</label>
                    <div class="p-3 rounded-lg bg-gray-50 dark:bg-white/5 text-sm">
                        @if($this->backupInfo)
                            <span class="font-medium text-gray-950 dark:text-white">{{ $this->backupInfo->name }}</span>
                            <span class="text-gray-500 ml-2">{{ $this->backupInfo->created_at?->format('M j, Y H:i') }}</span>
                            <span class="text-gray-500 ml-2">{{ $this->backupInfo->size_bytes > 0 ? \App\Support\Formatter::bytes($this->backupInfo->size_bytes) : '' }}</span>
                        @endif
                    </div>
                </div>

                <div class="flex justify-end">
                    <x-filament::button wire:click="nextStep" :disabled="empty($this->selectedUser)">
                        {{ __('Next: View Contents') }}
                    </x-filament::button>
                </div>
            </div>
        </x-filament::section>
    @endif

    {{-- Step 2: Show Contents --}}
    @if($this->step === 2)
        <x-filament::section>
            <x-slot name="heading">{{ __('Backup Contents for :user', ['user' => $this->selectedUser]) }}</x-slot>

            @if(empty($this->contents))
                <div class="text-center py-8 text-gray-500">{{ __('Loading...') }}</div>
            @else
                <div class="grid gap-4 md:grid-cols-3">
                    {{-- Files --}}
                    <button wire:click="selectSectionAndAdvance('files')" class="p-4 rounded-lg border border-gray-200 dark:border-white/10 hover:bg-gray-50 dark:hover:bg-white/5 text-left transition">
                        <div class="flex items-center gap-2 mb-2">
                            <x-heroicon-o-folder class="h-5 w-5 text-yellow-500" />
                            <span class="font-semibold text-gray-950 dark:text-white">{{ __('Files') }}</span>
                        </div>
                        <p class="text-sm text-gray-500">{{ count($this->contents['domains'] ?? []) }} {{ __('domain(s)') }}</p>
                        @foreach(($this->contents['domains'] ?? []) as $domain)
                            <x-filament::badge color="info" class="mt-1">{{ $domain }}</x-filament::badge>
                        @endforeach
                    </button>

                    {{-- Databases --}}
                    <button wire:click="selectSectionAndAdvance('databases')" class="p-4 rounded-lg border border-gray-200 dark:border-white/10 hover:bg-gray-50 dark:hover:bg-white/5 text-left transition">
                        <div class="flex items-center gap-2 mb-2">
                            <x-heroicon-o-circle-stack class="h-5 w-5 text-blue-500" />
                            <span class="font-semibold text-gray-950 dark:text-white">{{ __('Databases') }}</span>
                        </div>
                        <p class="text-sm text-gray-500">{{ count($this->contents['databases'] ?? []) }} {{ __('database(s)') }}</p>
                        @foreach(($this->contents['databases'] ?? []) as $db)
                            <x-filament::badge color="warning" class="mt-1">{{ $db }}</x-filament::badge>
                        @endforeach
                    </button>

                    {{-- Email --}}
                    <button wire:click="selectSectionAndAdvance('mailboxes')" class="p-4 rounded-lg border border-gray-200 dark:border-white/10 hover:bg-gray-50 dark:hover:bg-white/5 text-left transition">
                        <div class="flex items-center gap-2 mb-2">
                            <x-heroicon-o-envelope class="h-5 w-5 text-green-500" />
                            <span class="font-semibold text-gray-950 dark:text-white">{{ __('Email') }}</span>
                        </div>
                        <p class="text-sm text-gray-500">{{ count($this->contents['mailboxes'] ?? []) }} {{ __('mailbox(es)') }}</p>
                        @foreach(($this->contents['mailboxes'] ?? []) as $mbox)
                            <x-filament::badge color="gray" class="mt-1">{{ $mbox }}</x-filament::badge>
                        @endforeach
                    </button>
                </div>

                <div class="flex justify-between mt-4">
                    <x-filament::button color="gray" wire:click="prevStep">{{ __('Back') }}</x-filament::button>
                    <x-filament::button wire:click="nextStep">{{ __('Next: Select All & Skip to Confirm') }}</x-filament::button>
                </div>
            @endif
        </x-filament::section>
    @endif

    {{-- Step 3: Browse & Select Items --}}
    @if($this->step === 3)
        {{-- Section tabs --}}
        <x-filament::tabs>
            <x-filament::tabs.item :active="$this->activeSection === 'files'" wire:click="setSection('files')" icon="heroicon-o-folder">
                {{ __('Files') }}
            </x-filament::tabs.item>
            <x-filament::tabs.item :active="$this->activeSection === 'databases'" wire:click="setSection('databases')" icon="heroicon-o-circle-stack">
                {{ __('Databases') }}
            </x-filament::tabs.item>
            <x-filament::tabs.item :active="$this->activeSection === 'mailboxes'" wire:click="setSection('mailboxes')" icon="heroicon-o-envelope">
                {{ __('Email') }}
            </x-filament::tabs.item>
        </x-filament::tabs>

        @if($this->activeSection === 'files')
            {{-- File browser --}}
            <x-filament::section>
                <x-slot name="heading">
                    <div class="flex items-center justify-between">
                        <div class="flex items-center gap-2">
                            <x-heroicon-o-folder-open class="h-5 w-5" />
                            <span>{{ $this->currentPath ?: '/' }}</span>
                        </div>
                        @if(count($this->selectedPaths) > 0)
                            <x-filament::badge color="primary">{{ count($this->selectedPaths) }} {{ __('selected') }}</x-filament::badge>
                        @endif
                    </div>
                </x-slot>

                <div class="divide-y divide-gray-200 dark:divide-white/10">
                    @forelse($this->directoryItems as $idx => $item)
                        @if(($item['is_parent'] ?? false))
                            <button wire:key="dir-parent" wire:click="navigateTo('{{ $item['path'] }}')" class="flex w-full items-center gap-3 px-3 py-2 text-left text-sm hover:bg-gray-50 dark:hover:bg-white/5 transition">
                                <x-heroicon-o-arrow-up class="h-4 w-4 text-gray-400 shrink-0" />
                                <span class="font-medium text-gray-950 dark:text-white">..</span>
                            </button>
                        @else
                            <div wire:key="dir-item-{{ $idx }}" class="flex items-center gap-3 px-3 py-2 text-sm hover:bg-gray-50 dark:hover:bg-white/5">
                                <input type="checkbox" value="{{ $item['path'] }}" wire:model="selectedPaths" class="rounded border-gray-300 text-primary-600 dark:border-gray-600 dark:bg-gray-700" />
                                @if($item['is_dir'] ?? false)
                                    <button wire:click="navigateTo('{{ $item['path'] }}')" class="flex items-center gap-2 flex-1 text-left">
                                        <x-heroicon-o-folder class="h-4 w-4 text-yellow-500 shrink-0" />
                                        <span class="font-medium text-gray-950 dark:text-white">{{ $item['name'] }}</span>
                                    </button>
                                @else
                                    <x-heroicon-o-document class="h-4 w-4 text-gray-400 shrink-0" />
                                    <span class="text-gray-700 dark:text-gray-300 flex-1">{{ $item['name'] }}</span>
                                    @if($item['size'] ?? null)
                                        <span class="text-xs text-gray-500">{{ \App\Support\Formatter::bytes($item['size']) }}</span>
                                    @endif
                                @endif
                            </div>
                        @endif
                    @empty
                        <div class="px-3 py-8 text-center text-sm text-gray-500">{{ __('Empty directory') }}</div>
                    @endforelse
                </div>
            </x-filament::section>

        @elseif($this->activeSection === 'databases')
            <x-filament::section>
                <x-slot name="heading">{{ __('Select Databases') }}</x-slot>
                <div class="space-y-2">
                    @forelse($this->contents['databases'] ?? [] as $db)
                        <label class="flex items-center gap-3 px-3 py-2 rounded-lg hover:bg-gray-50 dark:hover:bg-white/5 cursor-pointer">
                            <input type="checkbox" value="{{ $db }}" wire:model.live="selectedDatabases" class="rounded border-gray-300 text-primary-600 dark:border-gray-600 dark:bg-gray-700" />
                            <x-heroicon-o-circle-stack class="h-4 w-4 text-blue-500" />
                            <span class="text-sm font-medium text-gray-950 dark:text-white">{{ $db }}</span>
                        </label>
                    @empty
                        <p class="text-sm text-gray-500 py-4 text-center">{{ __('No databases in this backup') }}</p>
                    @endforelse
                </div>
            </x-filament::section>

        @elseif($this->activeSection === 'mailboxes')
            <x-filament::section>
                <x-slot name="heading">{{ __('Select Mailboxes') }}</x-slot>
                <div class="space-y-2">
                    @forelse($this->contents['mailboxes'] ?? [] as $mbox)
                        <label class="flex items-center gap-3 px-3 py-2 rounded-lg hover:bg-gray-50 dark:hover:bg-white/5 cursor-pointer">
                            <input type="checkbox" value="{{ $mbox }}" wire:model.live="selectedMailboxes" class="rounded border-gray-300 text-primary-600 dark:border-gray-600 dark:bg-gray-700" />
                            <x-heroicon-o-envelope class="h-4 w-4 text-green-500" />
                            <span class="text-sm font-medium text-gray-950 dark:text-white">{{ $mbox }}</span>
                        </label>
                    @empty
                        <p class="text-sm text-gray-500 py-4 text-center">{{ __('No mailboxes in this backup') }}</p>
                    @endforelse
                </div>
            </x-filament::section>
        @endif

        <div class="flex justify-between mt-4">
            <x-filament::button color="gray" wire:click="prevStep">{{ __('Back') }}</x-filament::button>
            <x-filament::button wire:click="nextStep">{{ __('Next: Confirm') }}</x-filament::button>
        </div>
    @endif

    {{-- Step 4: Confirm & Restore --}}
    @if($this->step === 4)
        <x-filament::section>
            <x-slot name="heading">{{ __('Confirm Restore') }}</x-slot>

            <div class="space-y-4">
                <div class="grid gap-4 md:grid-cols-2">
                    <div>
                        <p class="text-sm font-medium text-gray-500 dark:text-gray-400">{{ __('User') }}</p>
                        <p class="font-semibold text-gray-950 dark:text-white">{{ $this->selectedUser }}</p>
                    </div>
                    <div>
                        <p class="text-sm font-medium text-gray-500 dark:text-gray-400">{{ __('Backup') }}</p>
                        <p class="font-semibold text-gray-950 dark:text-white">{{ $this->backupInfo?->name }}</p>
                    </div>
                </div>

                <div class="border-t border-gray-200 dark:border-white/10 pt-4">
                    <p class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ __('Items to restore:') }}</p>

                    @if(!empty($this->selectedPaths))
                        <div class="mb-2">
                            <span class="text-sm text-gray-500">{{ __('Files/Folders:') }}</span>
                            @foreach($this->selectedPaths as $path)
                                <x-filament::badge color="info" class="ml-1">{{ basename($path) }}</x-filament::badge>
                            @endforeach
                        </div>
                    @elseif(!empty($this->contents['domains']))
                        <div class="mb-2">
                            <span class="text-sm text-gray-500">{{ __('All domains:') }}</span>
                            @foreach($this->contents['domains'] as $d)
                                <x-filament::badge color="info" class="ml-1">{{ $d }}</x-filament::badge>
                            @endforeach
                        </div>
                    @endif

                    @if(!empty($this->selectedDatabases))
                        <div class="mb-2">
                            <span class="text-sm text-gray-500">{{ __('Databases:') }}</span>
                            @foreach($this->selectedDatabases as $db)
                                <x-filament::badge color="warning" class="ml-1">{{ $db }}</x-filament::badge>
                            @endforeach
                        </div>
                    @endif

                    @if(!empty($this->selectedMailboxes))
                        <div class="mb-2">
                            <span class="text-sm text-gray-500">{{ __('Mailboxes:') }}</span>
                            @foreach($this->selectedMailboxes as $m)
                                <x-filament::badge color="gray" class="ml-1">{{ $m }}</x-filament::badge>
                            @endforeach
                        </div>
                    @endif
                </div>

                <div class="border-t border-gray-200 dark:border-white/10 pt-4">
                    <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{ __('Conflict resolution') }}</label>
                    <select wire:model="conflictMode" class="w-full rounded-lg border-gray-300 dark:border-white/10 dark:bg-white/5 dark:text-white">
                        <option value="overwrite">{{ __('Overwrite existing files') }}</option>
                        <option value="skip">{{ __('Skip existing files') }}</option>
                    </select>
                </div>

                <div class="rounded-lg bg-yellow-50 p-3 dark:bg-yellow-950/20">
                    <p class="text-sm text-yellow-700 dark:text-yellow-400">
                        {{ __('Warning: This will restore the selected items. Existing files may be overwritten depending on the conflict resolution setting.') }}
                    </p>
                </div>

                <div class="flex justify-between">
                    <x-filament::button color="gray" wire:click="prevStep">{{ __('Back') }}</x-filament::button>
                    <x-filament::button color="danger" wire:click="executeRestore" wire:confirm="{{ __('Are you sure you want to restore these items?') }}">
                        {{ __('Restore Now') }}
                    </x-filament::button>
                </div>
            </div>
        </x-filament::section>
    @endif

    <x-filament-actions::modals />
</x-filament-panels::page>
