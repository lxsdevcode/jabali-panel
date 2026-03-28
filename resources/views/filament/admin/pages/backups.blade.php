<x-filament-panels::page>
    {{-- Destinations & Schedules summary --}}
    @php
        $destinations = \App\Models\BackupDestination::where('is_server_backup', true)->get();
        $schedules = \App\Models\BackupSchedule::where('is_server_backup', true)->get();
    @endphp

    @if($destinations->isNotEmpty() || $schedules->isNotEmpty())
    <div class="grid gap-4 md:grid-cols-2">
        @if($destinations->isNotEmpty())
        <x-filament::section compact>
            <x-slot name="heading">{{ __('Destinations') }}</x-slot>
            <div class="space-y-2">
                @foreach($destinations as $dest)
                <div class="flex items-center justify-between text-sm">
                    <div class="flex items-center gap-2">
                        <x-filament::badge :color="$dest->test_status === 'success' ? 'success' : ($dest->test_status === 'failed' ? 'danger' : 'gray')">
                            {{ strtoupper($dest->type) }}
                        </x-filament::badge>
                        <span class="font-medium text-gray-950 dark:text-white">{{ $dest->name }}</span>
                    </div>
                    <span class="text-xs text-gray-500">
                        {{ $dest->test_status === 'success' ? __('Connected') : ($dest->test_status === 'failed' ? __('Failed') : __('Not tested')) }}
                    </span>
                </div>
                @endforeach
            </div>
        </x-filament::section>
        @endif

        @if($schedules->isNotEmpty())
        <x-filament::section compact>
            <x-slot name="heading">{{ __('Schedules') }}</x-slot>
            <div class="space-y-2">
                @foreach($schedules as $schedule)
                <div class="flex items-center justify-between text-sm">
                    <div class="flex items-center gap-2">
                        <x-filament::badge :color="$schedule->is_active ? 'success' : 'gray'">
                            {{ $schedule->frequency_label }}
                        </x-filament::badge>
                        <span class="font-medium text-gray-950 dark:text-white">{{ $schedule->name }}</span>
                    </div>
                    <span class="text-xs text-gray-500">
                        {{ $schedule->time }} · {{ __('Keep :n', ['n' => $schedule->retention_count]) }}
                    </span>
                </div>
                @endforeach
            </div>
        </x-filament::section>
        @endif
    </div>
    @endif

    {{ $this->table }}

    <x-filament-actions::modals />
</x-filament-panels::page>
