<x-filament-widgets::widget>
    <x-filament::section>
        <x-slot name="heading">{{ __('Usage') }}</x-slot>

        @php
            $disk = $this->getDiskData();
            $bw = $this->getBandwidthData();
        @endphp

        <div class="grid gap-6 md:grid-cols-2">
            {{-- Disk Usage --}}
            <div>
                <div class="mb-3">
                    <span class="text-sm font-semibold text-gray-950 dark:text-white">{{ __('Disk Usage') }}</span>
                    <span class="text-xs text-gray-500 dark:text-gray-400 ml-2">{{ $disk['home'] }}</span>
                </div>

                @php
                    $diskPercent = min(100, max(0, $disk['percent']));
                    $diskColor = match($disk['color']) {
                        'danger' => 'bg-red-500',
                        'warning' => 'bg-amber-500',
                        default => 'bg-emerald-500',
                    };
                    $diskTrack = 'bg-gray-200 dark:bg-gray-700';
                @endphp

                <div class="w-full rounded-full h-4 {{ $diskTrack }}">
                    <div class="{{ $diskColor }} h-4 rounded-full transition-all" style="width: {{ $diskPercent }}%"></div>
                </div>

                <div class="flex flex-wrap items-center gap-4 mt-2 text-sm">
                    <span><strong>{{ $disk['used'] }}</strong> <span class="text-gray-500 dark:text-gray-400">{{ __('Used') }}</span></span>
                    @if($disk['free'])
                        <span><strong>{{ $disk['free'] }}</strong> <span class="text-gray-500 dark:text-gray-400">{{ __('Free') }}</span></span>
                    @endif
                    <span><strong>{{ $disk['quota'] }}</strong> <span class="text-gray-500 dark:text-gray-400">{{ __('Quota') }}</span></span>
                </div>
            </div>

            {{-- Bandwidth --}}
            <div>
                <div class="mb-3">
                    <span class="text-sm font-semibold text-gray-950 dark:text-white">{{ __('Bandwidth') }}</span>
                    <span class="text-xs text-gray-500 dark:text-gray-400 ml-2">{{ $bw['period'] }}</span>
                </div>

                @php
                    $bwPercent = min(100, max(0, $bw['percent']));
                    $bwColor = match($bw['color']) {
                        'danger' => 'bg-red-500',
                        'warning' => 'bg-amber-500',
                        default => 'bg-emerald-500',
                    };
                    $bwTrack = 'bg-gray-200 dark:bg-gray-700';
                @endphp

                <div class="w-full rounded-full h-4 {{ $bwTrack }}">
                    <div class="{{ $bwColor }} h-4 rounded-full transition-all" style="width: {{ $bw['has_quota'] ? $bwPercent : 0 }}%"></div>
                </div>

                <div class="flex flex-wrap items-center gap-4 mt-2 text-sm">
                    <span><strong>{{ $bw['used'] }}</strong> <span class="text-gray-500 dark:text-gray-400">{{ __('Used') }}</span></span>
                    @if($bw['free'])
                        <span><strong>{{ $bw['free'] }}</strong> <span class="text-gray-500 dark:text-gray-400">{{ __('Free') }}</span></span>
                    @endif
                    <span><strong>{{ $bw['quota'] }}</strong> <span class="text-gray-500 dark:text-gray-400">{{ __('Quota') }}</span></span>
                </div>
            </div>
        </div>
    </x-filament::section>
</x-filament-widgets::widget>
